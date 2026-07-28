package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) SaveOAuthState(ctx context.Context, digest [32]byte, expiresAt time.Time) error {
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO oauth_states (token_digest, expires_at)
		VALUES ($1, $2)
	`, digest[:], expiresAt); err != nil {
		return fmt.Errorf("insert OAuth state: %w", err)
	}
	return nil
}

func (store *PostgresStore) ConsumeOAuthState(ctx context.Context, digest [32]byte, now time.Time) (bool, error) {
	var valid bool
	err := store.pool.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE token_digest = $1
		RETURNING expires_at > $2
	`, digest[:], now).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("consume OAuth state: %w", err)
	}
	return valid, nil
}

func (store *PostgresStore) SaveSession(ctx context.Context, digest [32]byte, session Session, expiresAt time.Time) error {
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO owner_sessions (
			token_digest,
			github_user_id,
			github_login,
			github_avatar_url,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, digest[:], session.Owner.GitHubID, session.Owner.Login, session.Owner.AvatarURL, expiresAt); err != nil {
		return fmt.Errorf("insert owner session: %w", err)
	}
	return nil
}

func (store *PostgresStore) FindSession(ctx context.Context, digest [32]byte, now time.Time) (Session, error) {
	var session Session
	err := store.pool.QueryRow(ctx, `
		SELECT github_user_id, github_login, github_avatar_url, expires_at
		FROM owner_sessions
		WHERE token_digest = $1 AND expires_at > $2
	`, digest[:], now).Scan(
		&session.Owner.GitHubID,
		&session.Owner.Login,
		&session.Owner.AvatarURL,
		&session.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("query owner session: %w", err)
	}
	return session, nil
}

func (store *PostgresStore) DeleteSession(ctx context.Context, digest [32]byte) error {
	if _, err := store.pool.Exec(ctx, `DELETE FROM owner_sessions WHERE token_digest = $1`, digest[:]); err != nil {
		return fmt.Errorf("delete owner session: %w", err)
	}
	return nil
}
