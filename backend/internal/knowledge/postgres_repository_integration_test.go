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

func TestPostgresRepositoryPersistsDirectoryHierarchyAndDetectsCycles(t *testing.T) {
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
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	if err := repository.CreateSpace(ctx, space); err != nil {
		t.Fatalf("CreateSpace() error = %v", err)
	}
	root, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	child, _ := NewDirectory("33333333-3333-4333-8333-333333333333", space.ID(), root.ID(), "Architecture")
	if err := repository.CreateDirectory(ctx, root); err != nil {
		t.Fatalf("create root directory: %v", err)
	}
	if err := repository.CreateDirectory(ctx, child); err != nil {
		t.Fatalf("create child directory: %v", err)
	}

	directories, err := repository.ListDirectories(ctx, space.ID())
	if err != nil {
		t.Fatalf("ListDirectories() error = %v", err)
	}
	if len(directories) != 2 {
		t.Fatalf("directory count = %d, want 2", len(directories))
	}
	wouldCycle, err := repository.WouldCreateDirectoryCycle(ctx, root.ID(), child.ID())
	if err != nil {
		t.Fatalf("WouldCreateDirectoryCycle() error = %v", err)
	}
	if !wouldCycle {
		t.Error("moving root below its child was not detected as a cycle")
	}

	duplicate, _ := NewDirectory("44444444-4444-4444-8444-444444444444", space.ID(), root.ID(), "architecture")
	if err := repository.CreateDirectory(ctx, duplicate); !errors.Is(err, ErrDirectoryNameConflict) {
		t.Fatalf("duplicate CreateDirectory() error = %v, want %v", err, ErrDirectoryNameConflict)
	}
}

func TestPostgresRepositoryPersistsGlobalTags(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE tags`); err != nil {
		t.Fatalf("truncate tags: %v", err)
	}

	repository := NewPostgresRepository(pool)
	tag, _ := NewTag("11111111-1111-4111-8111-111111111111", "memory")
	if err := repository.CreateTag(ctx, tag); err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	tags, total, err := repository.ListTags(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}
	if len(tags) != 1 || total != 1 || tags[0].Name() != "memory" {
		t.Errorf("tags = %+v, total = %d", tags, total)
	}

	duplicate, _ := NewTag("22222222-2222-4222-8222-222222222222", "Memory")
	if err := repository.CreateTag(ctx, duplicate); !errors.Is(err, ErrTagNameConflict) {
		t.Fatalf("duplicate CreateTag() error = %v, want %v", err, ErrTagNameConflict)
	}
}
