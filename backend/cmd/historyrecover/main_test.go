package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ken-Chy129/note-foundry/backend/internal/historyimport"
)

var commandTestPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

func TestRunDefaultsToRecoveryBundlePreflight(t *testing.T) {
	bundle := writeCommandTestBundle(t)
	var output bytes.Buffer
	if err := run(context.Background(), []string{"-bundle", bundle}, &output, func(string) (string, bool) { return "", false }); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(output.String(), "preflight=ok") || !strings.Contains(output.String(), "documents=1") || !strings.Contains(output.String(), "attachments=1") || !strings.Contains(output.String(), "bundle_sha256=") {
		t.Fatalf("run() output = %q", output.String())
	}
}

func TestRunRequiresRuntimeConfigurationForApply(t *testing.T) {
	err := run(context.Background(), []string{"-bundle", writeCommandTestBundle(t), "-apply"}, io.Discard, func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("run() error = %v, want DATABASE_URL requirement", err)
	}
}

func writeCommandTestBundle(t *testing.T) string {
	t.Helper()
	digest := sha256.Sum256(commandTestPNG)
	manifest := historyimport.RecoveryManifest{
		Version: historyimport.RecoveryManifestVersion,
		Documents: []historyimport.RecoveryDocument{{
			Key: "note-key", NoteID: "note-id", Title: "Note", Confidence: historyimport.RecoveryConfidenceHigh,
			Mappings: []historyimport.RecoveryMapping{{MissingReference: "assets/image.png", File: "files/image.png", SHA256Hex: hex.EncodeToString(digest[:])}},
		}},
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "files"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "files", "image.png"), commandTestPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle
}
