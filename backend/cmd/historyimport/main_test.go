package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStagesHistoryDirectory(t *testing.T) {
	source := t.TempDir()
	output := filepath.Join(t.TempDir(), "staged")
	if err := os.WriteFile(filepath.Join(source, "安全点.md"), []byte("# 安全点\n\n正文\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"-source", source, "-output", output}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "documents=1") {
		t.Fatalf("run() stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(output, "manifest.json")); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
}

func TestRunRequiresSourceAndOutput(t *testing.T) {
	for _, arguments := range [][]string{{}, {"-source", t.TempDir()}} {
		if err := run(context.Background(), arguments, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%v) error = nil", arguments)
		}
	}
}
