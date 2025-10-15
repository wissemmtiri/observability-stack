package main

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Order struct {
	ID        int64
	Item      string
	Quantity  int
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func initDB(dbURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		item TEXT NOT NULL,
		quantity INT NOT NULL CHECK (quantity > 0),
		status TEXT NOT NULL DEFAULT 'received',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createOrder(ctx context.Context, db *sql.DB, item string, qty int) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
	INSERT INTO orders (item, quantity, status)
	VALUES ($1, $2, 'received')
	RETURNING id
	`, item, qty).Scan(&id)
	return id, err
}
