package backup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEncryptedArchiveRoundTripRestoresDatabaseAndAttachments(t *testing.T) {
	sourceAttachments := filepath.Join(t.TempDir(), "attachments")
	if err := os.MkdirAll(filepath.Join(sourceAttachments, "22"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceAttachments, "22", "diagram.svg"), []byte("<svg>Hermes</svg>"), 0o600); err != nil {
		t.Fatal(err)
	}
	tools := &databaseToolsStub{dumpContents: []byte("postgres dump with Hermes note")}
	createdAt := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	var encrypted bytes.Buffer
	manifest, err := (Archiver{
		DatabaseURL:          "postgres://source",
		AttachmentsDirectory: sourceAttachments,
		Passphrase:           "correct horse battery staple",
		Tools:                tools,
	}).Create(context.Background(), &encrypted, createdAt)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if manifest.AttachmentCount != 1 {
		t.Errorf("manifest = %+v", manifest)
	}
	if bytes.Contains(encrypted.Bytes(), []byte("Hermes")) || bytes.Contains(encrypted.Bytes(), tools.dumpContents) {
		t.Fatal("encrypted archive contains plaintext content")
	}

	targetAttachments := filepath.Join(t.TempDir(), "attachments")
	if err := os.MkdirAll(targetAttachments, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetAttachments, "stale.txt"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := (Restorer{
		DatabaseURL:          "postgres://target",
		AttachmentsDirectory: targetAttachments,
		Passphrase:           "correct horse battery staple",
		Tools:                tools,
	}).Restore(context.Background(), bytes.NewReader(encrypted.Bytes()))
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if !result.Manifest.CreatedAt.Equal(createdAt) {
		t.Errorf("restored manifest = %+v", result.Manifest)
	}
	if string(tools.restoredContents) != string(tools.dumpContents) || tools.restoredDatabaseURL != "postgres://target" {
		t.Errorf("restored database = %q to %q", tools.restoredContents, tools.restoredDatabaseURL)
	}
	attachment, err := os.ReadFile(filepath.Join(targetAttachments, "22", "diagram.svg"))
	if err != nil || string(attachment) != "<svg>Hermes</svg>" {
		t.Fatalf("restored Attachment = %q, %v", attachment, err)
	}
	if _, err := os.Stat(filepath.Join(targetAttachments, "stale.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale Attachment still exists: %v", err)
	}
}

func TestRestoreRejectsWrongPassphraseWithoutChangingAttachments(t *testing.T) {
	tools := &databaseToolsStub{dumpContents: []byte("database")}
	var encrypted bytes.Buffer
	if _, err := (Archiver{DatabaseURL: "source", AttachmentsDirectory: t.TempDir(), Passphrase: "right passphrase", Tools: tools}).Create(context.Background(), &encrypted, time.Now()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "attachments")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Restorer{DatabaseURL: "target", AttachmentsDirectory: target, Passphrase: "wrong passphrase", Tools: tools}).Restore(context.Background(), bytes.NewReader(encrypted.Bytes()))
	if err == nil || !strings.Contains(err.Error(), "decrypt backup") {
		t.Fatalf("Restore(wrong passphrase) error = %v", err)
	}
	if contents, readErr := os.ReadFile(filepath.Join(target, "keep.txt")); readErr != nil || string(contents) != "keep" {
		t.Fatalf("existing Attachment changed: %q, %v", contents, readErr)
	}
}

type databaseToolsStub struct {
	dumpContents        []byte
	restoredContents    []byte
	restoredDatabaseURL string
}

func (tools *databaseToolsStub) Dump(_ context.Context, _ string, destination string) error {
	return os.WriteFile(destination, tools.dumpContents, 0o600)
}

func (tools *databaseToolsStub) Restore(_ context.Context, databaseURL, source string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	tools.restoredContents = contents
	tools.restoredDatabaseURL = databaseURL
	return nil
}
