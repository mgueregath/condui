package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

func NewDatabase(path string) (*Database, error) {

	db, err := sql.Open(
		"sqlite",
		path,
	)

	if err != nil {
		return nil, err
	}

	return &Database{
		DB: db,
	}, nil
}
