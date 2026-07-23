// Package dbmanager is a thin, build-tag-driven indirection over the private
// db explorer submodule (backend/dbexplorer, repo mgueregath/navoro).
//
// Core app code (app.go, db_handlers.go, httpserver.go) only ever imports this
// package, never backend/dbexplorer directly, so the OSS build (no `dbmanager`
// tag, submodule absent) compiles cleanly with the feature disabled.
package dbmanager

import "errors"

// ErrNotAvailable is returned by Manager methods when the private db explorer
// submodule was not compiled in (build without the `dbmanager` tag).
var ErrNotAvailable = errors.New("db manager not available in this build")

// QueryColumn describes a single column returned by GetColumns.
type QueryColumn struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
	Key      string `json:"key"`
}

// QueryResult is the tabular result of a Query call.
type QueryResult struct {
	Columns  []string   `json:"columns"`
	Rows     [][]string `json:"rows"`
	RowCount int        `json:"rowCount"`
	Error    string     `json:"error"`
}

// Cred holds saved credentials for a given db type/port combination.
type Cred struct {
	User     string `json:"user"`
	Password string `json:"password"`
}
