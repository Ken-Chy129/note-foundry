package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ken-Chy129/note-foundry/backend/internal/historyimport"
)

func TestRunDefaultsToFilesystemPreflight(t *testing.T) {
	staging := t.TempDir()
	markdown := []byte("# Note\n\nBody\n")
	manifest := historyimport.StageManifest{
		Version: 1,
		Space:   historyimport.StageSpace{Name: "历史文档", Visibility: "private"},
		Documents: []historyimport.StageDocument{{
			Key:          "note-key",
			Title:        "Note",
			Directory:    []string{"Java 与 JVM", "Java"},
			MarkdownPath: "notes/note.md",
		}},
	}
	manifestContent, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staging, "notes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), manifestContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "notes", "note.md"), markdown, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err = run(context.Background(), []string{"-staging", staging}, &output, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(output.String(), "preflight=ok") || !strings.Contains(output.String(), "staging_sha256=") {
		t.Fatalf("run() output = %q", output.String())
	}
}

func TestRunRequiresRuntimeConfigurationForApply(t *testing.T) {
	staging := t.TempDir()
	manifest := historyimport.StageManifest{
		Version:   1,
		Space:     historyimport.StageSpace{Name: "历史文档", Visibility: "private"},
		Documents: []historyimport.StageDocument{{Key: "note-key", Title: "Note", MarkdownPath: "note.md"}},
	}
	content, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "note.md"), []byte("# Note\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := run(context.Background(), []string{"-staging", staging, "-apply"}, io.Discard, func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("run() error = %v, want DATABASE_URL requirement", err)
	}
}
