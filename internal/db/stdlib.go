package db

import (
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// openStdlib creates a *sql.DB from a pgxpool.Pool for compatibility with goose
func openStdlib(pool *pgxpool.Pool) (*sql.DB, error) {
	connConfig := pool.Config().ConnConfig.Copy()
	return sql.Open("pgx", stdlib.RegisterConnConfig(connConfig))
}

