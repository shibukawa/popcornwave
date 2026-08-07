package main

import (
	"context"
	"database/sql"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// store is the seam the two database layers plug into.
//
// It exists for one measurement. The Popcorn Wave service reaches PostgreSQL
// through database/sql wrapping pgx, and this service reached it through pgx's
// own pool, so a throughput difference between them could have been the
// framework or could have been those two layers. Selecting the layer with
// DB_DRIVER while every other line stays identical is what tells them apart.
type store interface {
	list(ctx context.Context) ([]Todo, error)
	create(ctx context.Context, title string) error
	toggle(ctx context.Context, id int64) error
	remove(ctx context.Context, id int64) error
	Close()
}

const (
	listSQL   = `SELECT id, title, done FROM todos ORDER BY id`
	createSQL = `INSERT INTO todos (title) VALUES ($1)`
	toggleSQL = `UPDATE todos SET done = NOT done WHERE id = $1`
	deleteSQL = `DELETE FROM todos WHERE id = $1`
)

// poolSize fixes both implementations at the same number of connections, so
// the comparison is of the layer and not of how many sockets it was given.
func poolSize() int32 {
	if v, err := strconv.Atoi(os.Getenv("DB_POOL")); err == nil && v > 0 {
		return int32(v)
	}
	return 25
}

// pgxStore is pgx's own pool, the arrangement a pgx user reaches for.
type pgxStore struct{ pool *pgxpool.Pool }

func newPgxStore(ctx context.Context, url string) (store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = poolSize()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &pgxStore{pool: pool}, nil
}

func (s *pgxStore) Close() { s.pool.Close() }

func (s *pgxStore) list(ctx context.Context) ([]Todo, error) {
	rows, err := s.pool.Query(ctx, listSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	todos := make([]Todo, 0, 32)
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (s *pgxStore) create(ctx context.Context, title string) error {
	_, err := s.pool.Exec(ctx, createSQL, title)
	return err
}

func (s *pgxStore) toggle(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, toggleSQL, id)
	return err
}

func (s *pgxStore) remove(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, deleteSQL, id)
	return err
}

// sqlStore is database/sql over the same pgx driver, which is the arrangement
// the framework uses.
type sqlStore struct{ db *sql.DB }

func newSQLStore(ctx context.Context, url string) (store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	n := int(poolSize())
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &sqlStore{db: db}, nil
}

func (s *sqlStore) Close() { _ = s.db.Close() }

func (s *sqlStore) list(ctx context.Context) ([]Todo, error) {
	rows, err := s.db.QueryContext(ctx, listSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	todos := make([]Todo, 0, 32)
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (s *sqlStore) create(ctx context.Context, title string) error {
	_, err := s.db.ExecContext(ctx, createSQL, title)
	return err
}

func (s *sqlStore) toggle(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, toggleSQL, id)
	return err
}

func (s *sqlStore) remove(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, deleteSQL, id)
	return err
}

// openStore selects the layer. Default is pgx's own pool, which is what a pgx
// user gets without thinking about it.
func openStore(ctx context.Context, url string) (store, string, error) {
	switch os.Getenv("DB_DRIVER") {
	case "", "pgxpool":
		s, err := newPgxStore(ctx, url)
		return s, "pgxpool", err
	case "sqldb":
		s, err := newSQLStore(ctx, url)
		return s, "database/sql+pgx", err
	default:
		s, err := newPgxStore(ctx, url)
		return s, "pgxpool", err
	}
}
