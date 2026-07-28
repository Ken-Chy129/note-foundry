package knowledge

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresRepositoryPersistsKnowledgeSpaces(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE knowledge_spaces CASCADE`); err != nil {
		t.Fatalf("truncate knowledge tables: %v", err)
	}

	repository := NewPostgresRepository(pool)
	space, err := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	if err != nil {
		t.Fatalf("NewSpace() error = %v", err)
	}
	if err := repository.CreateSpace(ctx, space); err != nil {
		t.Fatalf("CreateSpace() error = %v", err)
	}

	spaces, total, err := repository.ListSpaces(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListSpaces() error = %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("space count = %d, want 1", len(spaces))
	}
	if total != 1 {
		t.Errorf("total space count = %d, want 1", total)
	}
	if spaces[0].ID() != space.ID() || spaces[0].Name() != "AI Agent" || spaces[0].Visibility() != VisibilityPublic {
		t.Errorf("space = id:%q name:%q visibility:%q", spaces[0].ID(), spaces[0].Name(), spaces[0].Visibility())
	}

	space.Rename("Agent Systems")
	if err := repository.UpdateSpace(ctx, space); err != nil {
		t.Fatalf("UpdateSpace() error = %v", err)
	}
	updated, err := repository.GetSpace(ctx, space.ID())
	if err != nil {
		t.Fatalf("GetSpace() error = %v", err)
	}
	if updated.Name() != "Agent Systems" {
		t.Errorf("updated name = %q", updated.Name())
	}
}

func TestPostgresRepositoryRejectsDuplicateSpaceNamesIgnoringCase(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE knowledge_spaces CASCADE`); err != nil {
		t.Fatalf("truncate knowledge tables: %v", err)
	}

	repository := NewPostgresRepository(pool)
	first, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	duplicate, _ := NewSpace("22222222-2222-4222-8222-222222222222", "ai agent", VisibilityPrivate)
	if err := repository.CreateSpace(ctx, first); err != nil {
		t.Fatalf("first CreateSpace() error = %v", err)
	}
	if err := repository.CreateSpace(ctx, duplicate); !errors.Is(err, ErrSpaceNameConflict) {
		t.Fatalf("duplicate CreateSpace() error = %v, want %v", err, ErrSpaceNameConflict)
	}
}
