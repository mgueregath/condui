package models

import (
	"database/sql"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
)

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
	myDBs map[string]*sql.DB
	myMu  sync.Mutex
}
