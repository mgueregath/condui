package dbexplorer

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"
)

// DbConn holds an active SSH-tunneled database connection.
type DbConn struct {
	ID        string
	SessionID string
	DbType    string // "postgresql" | "mysql"
	Port      int
	User      string
	Password  string
	sshClient *ssh.Client
	// PostgreSQL: one *pgxpool.Pool per database (safe for concurrent access)
	pgPools map[string]*pgxpool.Pool
	pgMu    sync.Mutex
	// MySQL: one *sql.DB per database name
	myDBs            map[string]*sql.DB
	myMu             sync.Mutex
	sqliteDB         *sql.DB
	sqliteLocalPath  string
	sqliteRemotePath string
	sftpClient       *sftp.Client
}

// ConnectSQLite downloads a remote SQLite database to a private temporary file.
func (m *Manager) ConnectSQLite(client *sftp.Client, sessionID, remotePath string) (string, error) {
	if remotePath == "" {
		return "", fmt.Errorf("SQLite path is required")
	}
	remote, err := client.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer remote.Close()
	tmp, err := os.CreateTemp("", "condui-sqlite-*.db")
	if err != nil {
		return "", err
	}
	localPath := tmp.Name()
	if _, err = io.Copy(tmp, remote); err != nil {
		tmp.Close()
		os.Remove(localPath)
		return "", err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(localPath)
		return "", err
	}
	db, err := sql.Open("sqlite", localPath)
	if err != nil {
		os.Remove(localPath)
		return "", err
	}
	if err = db.Ping(); err != nil {
		db.Close()
		os.Remove(localPath)
		return "", err
	}
	_, _ = db.Exec("PRAGMA journal_mode=DELETE")
	id := uuid.New().String()
	conn := &DbConn{ID: id, SessionID: sessionID, DbType: "sqlite", sqliteDB: db, sqliteLocalPath: localPath, sqliteRemotePath: remotePath, sftpClient: client}
	m.mu.Lock()
	m.conns[id] = conn
	m.mu.Unlock()
	return id, nil
}

func (conn *DbConn) syncSQLite() error {
	if conn.sqliteDB == nil {
		return nil
	}
	if _, err := conn.sqliteDB.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil { /* DELETE mode may not support this */
	}
	local, err := os.Open(conn.sqliteLocalPath)
	if err != nil {
		return err
	}
	defer local.Close()
	remote, err := conn.sftpClient.OpenFile(conn.sqliteRemotePath, os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer remote.Close()
	_, err = io.Copy(remote, local)
	return err
}

// mysqlSSHClients maps DbConn.ID -> *ssh.Client for the custom condui-ssh dialer.
var mysqlSSHClients sync.Map

func init() {
	mysql.RegisterDialContext("condui-ssh", func(ctx context.Context, addr string) (net.Conn, error) {
		// addr is "<connID>@<host:port>"
		at := strings.LastIndex(addr, "@")
		if at < 0 {
			return nil, fmt.Errorf("invalid condui-ssh addr: %s", addr)
		}
		connID, realAddr := addr[:at], addr[at+1:]
		v, ok := mysqlSSHClients.Load(connID)
		if !ok {
			return nil, fmt.Errorf("no SSH client registered for conn %s", connID)
		}
		return v.(*ssh.Client).Dial("tcp", realAddr)
	})
}

// NormalizeDbType maps a free-form database type string to "postgresql" or "mysql".
func NormalizeDbType(t string) string {
	l := strings.ToLower(t)
	if strings.Contains(l, "postgres") || strings.Contains(l, "timescale") {
		return "postgresql"
	}
	if strings.Contains(l, "mysql") || strings.Contains(l, "mariadb") {
		return "mysql"
	}
	return l
}

// Manager tracks active SSH-tunneled database connections.
type Manager struct {
	mu    sync.Mutex
	conns map[string]*DbConn
}

// NewManager creates an empty connection manager.
func NewManager() *Manager {
	return &Manager{conns: make(map[string]*DbConn)}
}

func (m *Manager) get(id string) *DbConn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conns[id]
}

// Connect opens an SSH-tunneled database connection using native Go drivers.
func (m *Manager) Connect(client *ssh.Client, sessionID, dbType string, port int, user, password string) (string, error) {
	id := uuid.New().String()
	conn := &DbConn{
		ID:        id,
		SessionID: sessionID,
		DbType:    NormalizeDbType(dbType),
		Port:      port,
		User:      user,
		Password:  password,
		sshClient: client,
	}

	switch conn.DbType {
	case "postgresql":
		conn.pgPools = make(map[string]*pgxpool.Pool)
		// Test connection via pool ping
		pool, err := conn.pgGetPool("postgres")
		if err != nil {
			return "", err
		}
		if err := pool.Ping(context.Background()); err != nil {
			return "", err
		}

	case "mysql":
		conn.myDBs = make(map[string]*sql.DB)
		mysqlSSHClients.Store(id, client)
		db, err := conn.myGetDB("")
		if err != nil {
			mysqlSSHClients.Delete(id)
			return "", err
		}
		if err := db.PingContext(context.Background()); err != nil {
			mysqlSSHClients.Delete(id)
			return "", err
		}

	default:
		return "", fmt.Errorf("tipo de base de datos no soportado: %s", dbType)
	}

	m.mu.Lock()
	m.conns[id] = conn
	m.mu.Unlock()
	return id, nil
}

// Disconnect closes all driver connections and cleans up resources.
func (m *Manager) Disconnect(connID string) {
	m.mu.Lock()
	conn := m.conns[connID]
	delete(m.conns, connID)
	m.mu.Unlock()
	if conn == nil {
		return
	}
	conn.pgMu.Lock()
	for _, p := range conn.pgPools {
		p.Close()
	}
	conn.pgMu.Unlock()
	conn.myMu.Lock()
	for _, db := range conn.myDBs {
		db.Close()
	}
	conn.myMu.Unlock()
	if conn.DbType == "mysql" {
		mysqlSSHClients.Delete(connID)
	}
	if conn.DbType == "sqlite" {
		conn.sqliteDB.Close()
		_ = conn.syncSQLite()
		_ = os.Remove(conn.sqliteLocalPath)
	}
}
