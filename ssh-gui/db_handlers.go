package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ssh-gui/backend/dbmanager"
)

// ─── DB API helpers ──────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ─── DB API handlers ─────────────────────────────────────────────────────────

func (a *App) handleDbApiConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session  string `json:"session"`
		DbType   string `json:"dbType"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Path     string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	var connID string
	var err error
	normalized := dbmanager.NormalizeDbType(req.DbType)
	switch {
	case normalized == "sqlite":
		session, ok := a.sessionManager.Get(req.Session)
		if !ok || session.SFTP == nil {
			jsonErr(w, "SSH/SFTP session not found", 400)
			return
		}
		connID, err = a.dbExplorer.ConnectSQLite(session.SFTP, req.Session, req.Path)
	case dbmanager.IsKVEngine(normalized):
		connID, err = a.DbConnectKV(req.Session, req.DbType, req.Port, req.User, req.Password)
	default:
		connID, err = a.DbConnect(req.Session, req.DbType, req.Port, req.User, req.Password, req.Path)
	}
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"connId": connID})
}

func (a *App) handleDbApiDisconnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnID string `json:"connId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	a.DbDisconnect(req.ConnID)
	jsonOK(w, map[string]bool{"ok": true})
}

func (a *App) handleDbApiDatabases(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	dbs, err := a.DbListDatabases(connID)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, dbs)
}

func (a *App) handleDbApiSchemas(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schemas, err := a.DbListSchemas(connID, db)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, schemas)
}

func (a *App) handleDbApiTables(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	tables, err := a.DbListTables(connID, db, schema)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, tables)
}

func (a *App) handleDbApiColumns(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	cols, err := a.DbGetColumns(connID, db, schema, table)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, cols)
}

func (a *App) handleDbApiIndexes(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	idx, err := a.DbGetIndexes(connID, db, schema, table)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, idx)
}

func (a *App) handleDbApiForeignKeys(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	fks, err := a.DbGetForeignKeys(connID, db, schema, table)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, fks)
}

func (a *App) handleDbApiQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnID string `json:"connId"`
		DB     string `json:"db"`
		SQL    string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	result, _ := a.DbQuery(req.ConnID, req.DB, req.SQL)
	jsonOK(w, result)
}

func (a *App) handleDbApiCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		dbType := r.URL.Query().Get("type")
		port := r.URL.Query().Get("port")
		key := dbType + ":" + port
		creds := dbmanager.LoadCreds()
		if c, ok := creds[key]; ok {
			jsonOK(w, c)
		} else {
			jsonOK(w, dbmanager.Cred{})
		}
	case http.MethodPost:
		var req struct {
			Type     string `json:"type"`
			Port     int    `json:"port"`
			User     string `json:"user"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		key := fmt.Sprintf("%s:%d", req.Type, req.Port)
		creds := dbmanager.LoadCreds()
		creds[key] = dbmanager.Cred{User: req.User, Password: req.Password}
		dbmanager.SaveCreds(creds)
		jsonOK(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		var req struct {
			Type string `json:"type"`
			Port int    `json:"port"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		key := fmt.Sprintf("%s:%d", req.Type, req.Port)
		creds := dbmanager.LoadCreds()
		delete(creds, key)
		dbmanager.SaveCreds(creds)
		jsonOK(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, "method not allowed", 405)
	}
}

// ─── DB Explorer Wails-bound methods ─────────────────────────────────────────

// DbConnect opens an SSH-tunneled database connection using native Go drivers.
func (a *App) DbConnect(sessionID, dbType string, port int, user, password, database string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("SSH session not found")
	}
	return a.dbExplorer.Connect(session.Client, sessionID, dbType, port, user, password, database)
}

// DbConnectKV opens an SSH-tunneled key-value connection (Redis).
func (a *App) DbConnectKV(sessionID, dbType string, port int, user, password string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("SSH session not found")
	}
	return a.dbExplorer.ConnectKV(session.Client, sessionID, dbType, port, user, password)
}

// DbDisconnect closes all driver connections and cleans up resources.
func (a *App) DbDisconnect(connID string) {
	a.dbExplorer.Disconnect(connID)
}

// DbListDatabases returns available databases over the SSH-tunneled connection.
func (a *App) DbListDatabases(connID string) ([]string, error) {
	return a.dbExplorer.ListDatabases(connID)
}

// DbListSchemas returns schemas for a given database.
func (a *App) DbListSchemas(connID, database string) ([]string, error) {
	return a.dbExplorer.ListSchemas(connID, database)
}

// DbListTables returns tables for a database/schema using parameterized queries.
func (a *App) DbListTables(connID, database, schema string) ([]string, error) {
	return a.dbExplorer.ListTables(connID, database, schema)
}

// DbGetColumns returns column metadata using parameterized queries.
func (a *App) DbGetColumns(connID, database, schema, table string) ([]dbmanager.QueryColumn, error) {
	return a.dbExplorer.GetColumns(connID, database, schema, table)
}

// DbQuery executes arbitrary SQL and returns the result.
func (a *App) DbQuery(connID, database, query string) (dbmanager.QueryResult, error) {
	return a.dbExplorer.Query(connID, database, query)
}

// DbGetIndexes returns index metadata for a table.
func (a *App) DbGetIndexes(connID, database, schema, table string) ([]dbmanager.Index, error) {
	return a.dbExplorer.GetIndexes(connID, database, schema, table)
}

// DbGetForeignKeys returns foreign key columns for a table.
func (a *App) DbGetForeignKeys(connID, database, schema, table string) ([]dbmanager.ForeignKey, error) {
	return a.dbExplorer.GetForeignKeys(connID, database, schema, table)
}

// DbKVListDatabases returns the key-value engine's databases (Redis:
// numbered DBs that have keys).
func (a *App) DbKVListDatabases(connID string) ([]string, error) {
	return a.dbExplorer.KVListDatabases(connID)
}

// DbKVListKeys returns keys matching pattern ("" = "*") in a database.
func (a *App) DbKVListKeys(connID, database, pattern string) ([]string, error) {
	return a.dbExplorer.KVListKeys(connID, database, pattern)
}

// DbKVGetValue returns a single key's type/TTL/value.
func (a *App) DbKVGetValue(connID, database, key string) (dbmanager.KVValue, error) {
	return a.dbExplorer.KVGetValue(connID, database, key)
}

// DbKVSetValue sets a string key's value.
func (a *App) DbKVSetValue(connID, database, key, value string) error {
	return a.dbExplorer.KVSetValue(connID, database, key, value)
}

// DbKVDeleteKey deletes a key.
func (a *App) DbKVDeleteKey(connID, database, key string) error {
	return a.dbExplorer.KVDeleteKey(connID, database, key)
}

func (a *App) handleDbApiKVDatabases(w http.ResponseWriter, r *http.Request) {
	dbs, err := a.DbKVListDatabases(r.URL.Query().Get("connId"))
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, dbs)
}

func (a *App) handleDbApiKVKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.DbKVListKeys(r.URL.Query().Get("connId"), r.URL.Query().Get("db"), r.URL.Query().Get("pattern"))
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, keys)
}

func (a *App) handleDbApiKVValue(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		v, err := a.DbKVGetValue(r.URL.Query().Get("connId"), r.URL.Query().Get("db"), r.URL.Query().Get("key"))
		if err != nil {
			jsonErr(w, err.Error(), 400)
			return
		}
		jsonOK(w, v)
	case http.MethodPost:
		var req struct{ ConnID, DB, Key, Value string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, err.Error(), 400)
			return
		}
		if err := a.DbKVSetValue(req.ConnID, req.DB, req.Key, req.Value); err != nil {
			jsonErr(w, err.Error(), 400)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		var req struct{ ConnID, DB, Key string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, err.Error(), 400)
			return
		}
		if err := a.DbKVDeleteKey(req.ConnID, req.DB, req.Key); err != nil {
			jsonErr(w, err.Error(), 400)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, "method not allowed", 405)
	}
}
