package store

import (
	"context"
	"database/sql"
)

// Store implements PostgreSQL-backed application workflows.
type Store struct {
	db *sql.DB
}

// New constructs a Store using a caller-owned database pool.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) withinTx(
	ctx context.Context,
	fn func(*sql.Tx) error,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return translateDatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return translateDatabaseError(err)
	}
	return nil
}
