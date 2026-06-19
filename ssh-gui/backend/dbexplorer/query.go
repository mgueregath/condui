package dbexplorer

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssh-gui/backend/models"
)

// pgGetPool returns (creating if needed) a *pgxpool.Pool for the given database.
// Pools are safe for concurrent use, unlike *pgx.Conn.
func (conn *DbConn) pgGetPool(database string) (*pgxpool.Pool, error) {
	if database == "" {
		database = "postgres"
	}
	conn.pgMu.Lock()
	defer conn.pgMu.Unlock()

	if p, ok := conn.pgPools[database]; ok {
		return p, nil
	}

	cfg, err := pgxpool.ParseConfig(fmt.Sprintf(
		"host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable",
		conn.Port, conn.User, conn.Password, database))
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return conn.sshClient.Dial("tcp", addr)
	}
	cfg.MaxConns = 4

	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	conn.pgPools[database] = p
	return p, nil
}

// myGetDB returns (creating if needed) a *sql.DB for the given MySQL database.
func (conn *DbConn) myGetDB(database string) (*sql.DB, error) {
	conn.myMu.Lock()
	defer conn.myMu.Unlock()

	if db, ok := conn.myDBs[database]; ok {
		return db, nil
	}

	cfg := mysql.NewConfig()
	cfg.User = conn.User
	cfg.Passwd = conn.Password
	cfg.Net = "condui-ssh"
	cfg.Addr = fmt.Sprintf("%s@127.0.0.1:%d", conn.ID, conn.Port)
	cfg.DBName = database
	cfg.ParseTime = true
	cfg.MultiStatements = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	conn.myDBs[database] = db
	return db, nil
}

// pgExecResult runs sql on a pgxpool and returns a QueryResult.
func pgExecResult(pool *pgxpool.Pool, query string) models.QueryResult {
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	defer rows.Close()

	var res models.QueryResult
	for _, fd := range rows.FieldDescriptions() {
		res.Columns = append(res.Columns, fd.Name)
	}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			break
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				row[i] = ""
			} else {
				switch t := v.(type) {
				case []byte:
					row[i] = string(t)
				case time.Time:
					row[i] = t.Format(time.RFC3339)
				default:
					row[i] = fmt.Sprintf("%v", t)
				}
			}
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	res.RowCount = len(res.Rows)
	return res
}

// myExecResult runs sql on a *sql.DB and returns a QueryResult.
func myExecResult(db *sql.DB, query string) models.QueryResult {
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Non-SELECT DML/DDL: fall back to Exec
		result, execErr := db.ExecContext(ctx, query)
		if execErr != nil {
			return models.QueryResult{Error: execErr.Error()}
		}
		affected, _ := result.RowsAffected()
		return models.QueryResult{RowCount: int(affected)}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	var res models.QueryResult
	res.Columns = cols

	raw := make([]sql.RawBytes, len(cols))
	dest := make([]interface{}, len(cols))
	for i := range raw {
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			break
		}
		row := make([]string, len(cols))
		for i, v := range raw {
			if v == nil {
				row[i] = ""
			} else {
				row[i] = string(v)
			}
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	res.RowCount = len(res.Rows)
	return res
}

// execQuery dispatches a SQL query to the right driver connection.
func (conn *DbConn) execQuery(database, query string) models.QueryResult {
	switch conn.DbType {
	case "postgresql":
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return models.QueryResult{Error: err.Error()}
		}
		return pgExecResult(pool, query)
	case "mysql":
		db, err := conn.myGetDB(database)
		if err != nil {
			return models.QueryResult{Error: err.Error()}
		}
		return myExecResult(db, query)
	}
	return models.QueryResult{Error: "unsupported database type"}
}

// firstColumn runs a single-column query and returns the values.
func (conn *DbConn) firstColumn(database, query string) ([]string, error) {
	r := conn.execQuery(database, query)
	if r.Error != "" {
		return nil, fmt.Errorf("%s", r.Error)
	}
	var out []string
	for _, row := range r.Rows {
		if len(row) > 0 {
			out = append(out, row[0])
		}
	}
	return out, nil
}
