//go:build dbmanager

package dbmanager

import (
	"io/fs"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	dbexplorer "github.com/mgueregath/navoro/core"
)

// Enabled reports whether the private db explorer submodule was compiled in.
const Enabled = true

// Manager wraps the private dbexplorer.Manager.
type Manager struct {
	impl *dbexplorer.Manager
}

func NewManager() *Manager {
	return &Manager{impl: dbexplorer.NewManager()}
}

func NormalizeDbType(t string) string {
	return dbexplorer.NormalizeDbType(t)
}

func (m *Manager) ConnectSQLite(client *sftp.Client, sessionID, remotePath string) (string, error) {
	return m.impl.ConnectSQLite(client, sessionID, remotePath)
}

func (m *Manager) Connect(client *ssh.Client, sessionID, dbType string, port int, user, password, database string) (string, error) {
	return m.impl.Connect(client, sessionID, dbType, port, user, password, database)
}

func (m *Manager) Disconnect(connID string) {
	m.impl.Disconnect(connID)
}

func (m *Manager) ListDatabases(connID string) ([]string, error) {
	return m.impl.ListDatabases(connID)
}

func (m *Manager) ListSchemas(connID, database string) ([]string, error) {
	return m.impl.ListSchemas(connID, database)
}

func (m *Manager) ListTables(connID, database, schema string) ([]string, error) {
	return m.impl.ListTables(connID, database, schema)
}

func (m *Manager) GetColumns(connID, database, schema, table string) ([]QueryColumn, error) {
	cols, err := m.impl.GetColumns(connID, database, schema, table)
	if err != nil {
		return nil, err
	}
	out := make([]QueryColumn, len(cols))
	for i, c := range cols {
		out[i] = QueryColumn{Name: c.Name, DataType: c.DataType, Nullable: c.Nullable, Default: c.Default, Key: c.Key}
	}
	return out, nil
}

func (m *Manager) Query(connID, database, query string) (QueryResult, error) {
	r, err := m.impl.Query(connID, database, query)
	return QueryResult{Columns: r.Columns, Rows: r.Rows, RowCount: r.RowCount, Error: r.Error}, err
}

func (m *Manager) GetIndexes(connID, database, schema, table string) ([]Index, error) {
	idxs, err := m.impl.GetIndexes(connID, database, schema, table)
	if err != nil {
		return nil, err
	}
	out := make([]Index, len(idxs))
	for i, ix := range idxs {
		out[i] = Index{Name: ix.Name, Columns: ix.Columns, Unique: ix.Unique}
	}
	return out, nil
}

func (m *Manager) GetForeignKeys(connID, database, schema, table string) ([]ForeignKey, error) {
	fks, err := m.impl.GetForeignKeys(connID, database, schema, table)
	if err != nil {
		return nil, err
	}
	out := make([]ForeignKey, len(fks))
	for i, fk := range fks {
		out[i] = ForeignKey{Name: fk.Name, Column: fk.Column, RefTable: fk.RefTable, RefColumn: fk.RefColumn}
	}
	return out, nil
}

func LoadCreds() map[string]Cred {
	creds := dbexplorer.LoadCreds()
	out := make(map[string]Cred, len(creds))
	for k, c := range creds {
		out[k] = Cred{User: c.User, Password: c.Password}
	}
	return out
}

func SaveCreds(creds map[string]Cred) {
	out := make(map[string]dbexplorer.Cred, len(creds))
	for k, c := range creds {
		out[k] = dbexplorer.Cred{User: c.User, Password: c.Password}
	}
	dbexplorer.SaveCreds(out)
}

func IsKVEngine(normalizedType string) bool {
	return dbexplorer.IsKVEngine(normalizedType)
}

func (m *Manager) ConnectKV(client *ssh.Client, sessionID, dbType string, port int, user, password string) (string, error) {
	return m.impl.ConnectKV(client, sessionID, dbType, port, user, password)
}

func (m *Manager) KVListDatabases(connID string) ([]string, error) {
	return m.impl.KVListDatabases(connID)
}

func (m *Manager) KVListKeys(connID, database, pattern string) ([]string, error) {
	return m.impl.KVListKeys(connID, database, pattern)
}

func (m *Manager) KVGetValue(connID, database, key string) (KVValue, error) {
	v, err := m.impl.KVGetValue(connID, database, key)
	return KVValue{Type: v.Type, TTL: v.TTL, Value: v.Value}, err
}

func (m *Manager) KVSetValue(connID, database, key, value string) error {
	return m.impl.KVSetValue(connID, database, key, value)
}

func (m *Manager) KVDeleteKey(connID, database, key string) error {
	return m.impl.KVDeleteKey(connID, database, key)
}

func AssetsFS() (fs.FS, error) {
	return dbexplorer.AssetsFS()
}

func RenderIndexHTML(sessionID, dbType, portStr, remotePath, apiBase string) ([]byte, error) {
	return dbexplorer.RenderIndexHTML(sessionID, dbType, portStr, remotePath, apiBase)
}
