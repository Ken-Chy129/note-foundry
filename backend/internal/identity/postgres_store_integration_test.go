package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresStorePersistsOAuthStateAndOwnerSessions(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE oauth_states, owner_sessions`); err != nil {
		t.Fatalf("truncate identity tables: %v", err)
	}

	store := NewPostgresStore(pool)
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	stateDigest := sha256.Sum256([]byte("oauth-state"))
	if err := store.SaveOAuthState(ctx, stateDigest, now.Add(time.Minute)); err != nil {
		t.Fatalf("SaveOAuthState() error = %v", err)
	}
	valid, err := store.ConsumeOAuthState(ctx, stateDigest, now)
	if err != nil {
		t.Fatalf("ConsumeOAuthState() error = %v", err)
	}
	if !valid {
		t.Error("ConsumeOAuthState() valid = false, want true")
	}
	valid, err = store.ConsumeOAuthState(ctx, stateDigest, now)
	if err != nil {
		t.Fatalf("second ConsumeOAuthState() error = %v", err)
	}
	if valid {
		t.Error("OAuth state was accepted more than once")
	}

	sessionDigest := sha256.Sum256([]byte("session-token"))
	wantSession := Session{
		Owner: Owner{
			GitHubID:  42,
			Login:     "knowledge-owner",
			AvatarURL: "https://avatars.test/owner",
		},
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := store.SaveSession(ctx, sessionDigest, wantSession, wantSession.ExpiresAt); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	gotSession, err := store.FindSession(ctx, sessionDigest, now)
	if err != nil {
		t.Fatalf("FindSession() error = %v", err)
	}
	if gotSession.Owner != wantSession.Owner {
		t.Errorf("owner = %+v, want %+v", gotSession.Owner, wantSession.Owner)
	}
	if !gotSession.ExpiresAt.Equal(wantSession.ExpiresAt) {
		t.Errorf("expiry = %v, want %v", gotSession.ExpiresAt, wantSession.ExpiresAt)
	}
	if err := store.DeleteSession(ctx, sessionDigest); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	_, err = store.FindSession(ctx, sessionDigest, now)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("FindSession() after delete error = %v, want %v", err, ErrSessionNotFound)
	}
}
