package database

import (
	"database/sql"
	"log"
	"time"

	turso "turso.tech/database/tursogo-serverless"
)

type DB struct {
	*sql.DB
}

func Connect(url, token string) (*DB, error) {
	db := sql.OpenDB(turso.NewConnector(url, token))

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✅ Connected to Turso")
	return &DB{db}, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}
