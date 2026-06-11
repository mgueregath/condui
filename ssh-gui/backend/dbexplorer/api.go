package dbexplorer

import (
	"context"
	"fmt"

	"ssh-gui/backend/models"
)

// ListDatabases returns available databases over the SSH-tunneled connection.
func (m *Manager) ListDatabases(connID string) ([]string, error) {
	conn := m.get(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	switch conn.DbType {
	case "postgresql":
		return conn.firstColumn("postgres",
			"SELECT datname FROM pg_database WHERE datistemplate=false ORDER BY datname")
	case "mysql":
		return conn.firstColumn("", "SHOW DATABASES")
	}
	return nil, fmt.Errorf("unsupported")
}

// ListSchemas returns schemas for a given database.
func (m *Manager) ListSchemas(connID, database string) ([]string, error) {
	conn := m.get(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	switch conn.DbType {
	case "postgresql":
		return conn.firstColumn(database,
			`SELECT schema_name FROM information_schema.schemata
			 WHERE schema_name NOT IN ('pg_catalog','information_schema','pg_toast')
			   AND schema_name NOT LIKE 'pg_temp%' AND schema_name NOT LIKE 'pg_toast_temp%'
			 ORDER BY schema_name`)
	case "mysql":
		return []string{database}, nil
	}
	return nil, fmt.Errorf("unsupported")
}

// ListTables returns tables for a database/schema using parameterized queries.
func (m *Manager) ListTables(connID, database, schema string) ([]string, error) {
	conn := m.get(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	ctx := context.Background()
	switch conn.DbType {
	case "postgresql":
		if schema == "" {
			schema = "public"
		}
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return nil, err
		}
		rows, err := pool.Query(ctx,
			`SELECT table_name FROM information_schema.tables
			 WHERE table_schema=$1 AND table_type='BASE TABLE'
			 ORDER BY table_name`, schema)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var t string
			rows.Scan(&t)
			tables = append(tables, t)
		}
		return tables, rows.Err()
	case "mysql":
		return conn.firstColumn(database, "SHOW TABLES")
	}
	return nil, fmt.Errorf("unsupported")
}

// GetColumns returns column metadata using parameterized queries.
func (m *Manager) GetColumns(connID, database, schema, table string) ([]models.QueryColumn, error) {
	conn := m.get(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	ctx := context.Background()
	switch conn.DbType {
	case "postgresql":
		if schema == "" {
			schema = "public"
		}
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return nil, err
		}
		rows, err := pool.Query(ctx, `
			SELECT c.column_name, c.data_type,
			       c.is_nullable='YES',
			       COALESCE(c.column_default,''),
			       CASE WHEN pk.column_name IS NOT NULL THEN 'PK' ELSE '' END
			FROM information_schema.columns c
			LEFT JOIN (
			    SELECT kcu.column_name
			    FROM information_schema.table_constraints tc
			    JOIN information_schema.key_column_usage kcu
			      ON tc.constraint_name=kcu.constraint_name AND tc.table_schema=kcu.table_schema
			    WHERE tc.constraint_type='PRIMARY KEY'
			      AND tc.table_schema=$1 AND tc.table_name=$2
			) pk ON c.column_name=pk.column_name
			WHERE c.table_schema=$1 AND c.table_name=$2
			ORDER BY c.ordinal_position`, schema, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []models.QueryColumn
		for rows.Next() {
			var c models.QueryColumn
			rows.Scan(&c.Name, &c.DataType, &c.Nullable, &c.Default, &c.Key)
			cols = append(cols, c)
		}
		return cols, rows.Err()

	case "mysql":
		db, err := conn.myGetDB(database)
		if err != nil {
			return nil, err
		}
		rows, err := db.QueryContext(ctx, `
			SELECT COLUMN_NAME, DATA_TYPE,
			       IF(IS_NULLABLE='YES','1','0'),
			       COALESCE(COLUMN_DEFAULT,''),
			       COLUMN_KEY
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=? AND TABLE_NAME=?
			ORDER BY ORDINAL_POSITION`, database, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []models.QueryColumn
		for rows.Next() {
			var c models.QueryColumn
			var nullableStr string
			rows.Scan(&c.Name, &c.DataType, &nullableStr, &c.Default, &c.Key)
			c.Nullable = nullableStr == "1"
			cols = append(cols, c)
		}
		return cols, rows.Err()
	}
	return nil, fmt.Errorf("unsupported")
}

// Query executes arbitrary SQL and returns the result.
func (m *Manager) Query(connID, database, query string) (models.QueryResult, error) {
	conn := m.get(connID)
	if conn == nil {
		return models.QueryResult{Error: "connection not found"}, nil
	}
	return conn.execQuery(database, query), nil
}
