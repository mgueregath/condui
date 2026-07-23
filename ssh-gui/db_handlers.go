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
	if dbmanager.NormalizeDbType(req.DbType) == "sqlite" {
		session, ok := a.sessionManager.Get(req.Session)
		if !ok || session.SFTP == nil {
			jsonErr(w, "SSH/SFTP session not found", 400)
			return
		}
		connID, err = a.dbExplorer.ConnectSQLite(session.SFTP, req.Session, req.Path)
	} else {
		connID, err = a.DbConnect(req.Session, req.DbType, req.Port, req.User, req.Password)
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
func (a *App) DbConnect(sessionID, dbType string, port int, user, password string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("SSH session not found")
	}
	return a.dbExplorer.Connect(session.Client, sessionID, dbType, port, user, password)
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
