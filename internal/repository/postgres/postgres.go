// Package postgres stores subscriptions in PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

//go:embed schema.sql
var schema string

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Migrate creates the table if it does not exist yet. The schema is tiny,
// so a full migration tool would be overkill here.
func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

func (r *Repository) Upsert(ctx context.Context, sub domain.Subscription) error {
	const q = `
		INSERT INTO subscriptions (chat_id, github_login, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id) DO UPDATE
		SET github_login = EXCLUDED.github_login,
		    updated_at   = EXCLUDED.updated_at`

	if _, err := r.db.ExecContext(ctx, q, sub.ChatID, sub.GitHubLogin, sub.CreatedAt, sub.UpdatedAt); err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, chatID int64) (domain.Subscription, error) {
	const q = `SELECT chat_id, github_login, created_at, updated_at FROM subscriptions WHERE chat_id = $1`

	var s domain.Subscription
	err := r.db.QueryRowContext(ctx, q, chatID).Scan(&s.ChatID, &s.GitHubLogin, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Subscription{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("select subscription: %w", err)
	}
	return s, nil
}

func (r *Repository) Delete(ctx context.Context, chatID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE chat_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ChatIDsByLogin(ctx context.Context, login string) ([]int64, error) {
	const q = `SELECT chat_id FROM subscriptions WHERE lower(github_login) = lower($1) ORDER BY chat_id`

	rows, err := r.db.QueryContext(ctx, q, login)
	if err != nil {
		return nil, fmt.Errorf("select chats by login: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan chat id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
