package db

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func PSQL(connStr string) (DB, error) {
	pool, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}

	pool.SetMaxOpenConns(50)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(time.Hour)

	return &dB{pool, dbTypePSQL}, nil
}
