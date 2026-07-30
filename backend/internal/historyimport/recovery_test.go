package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/attachments"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
)

var recoveryTestPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89,
}

func TestApplyRecoveryUploadsAssetsAndAutosavesOnce(t *testing.T) {
	bundle := writeRecoveryTestBundle(t, []RecoveryDocument{{
		Key:        "note-key",
		NoteID:     "note-id",
		Title:      "Note",
		Confidence: RecoveryConfidenceHigh,
		Mappings: []RecoveryMapping{
			{MissingReference: "assets/first.png", File: "files/first.png", SHA256Hex: recoverySHA256(recoveryTestPNG)},
			{MissingReference: "assets/second.png", File: "files/second.png", SHA256Hex: recoverySHA256(recoveryTestPNG)},
		},
	}}, map[string][]byte{
		"files/first.png":  recoveryTestPNG,
		"files/second.png": recoveryTestPNG,
	})
	note, err := notes.NewNote("note-id", "space-id", "", "Note", "![one](assets/first.png)\n![two](assets/second.png)\n")
	if err != nil {
		t.Fatal(err)
	}
	noteService := &recoveryNotesFake{notes: map[string]*notes.Note{"note-id": note}}
	attachmentService := &recoveryAttachmentsFake{}

	summary, err := ApplyRecovery(context.Background(), bundle, noteService, attachmentService)
	if err != nil {
		t.Fatalf("ApplyRecovery() error = %v", err)
	}
	if summary.DocumentsPlanned != 1 || summary.AttachmentsPlanned != 2 || summary.AttachmentsUploaded != 2 || summary.DocumentsCompleted != 1 {
		t.Fatalf("ApplyRecovery() summary = %+v", summary)
	}
	if noteService.autosaves != 1 {
		t.Fatalf("autosaves = %d, want 1", noteService.autosaves)
	}
	if note.Markdown() != "![one](attachment:attachment-1)\n![two](attachment:attachment-2)\n" {
		t.Fatalf("Markdown = %q", note.Markdown())
	}
	if note.Published() != nil {
		t.Fatal("recovery unexpectedly published the note")
	}

	second, err := ApplyRecovery(context.Background(), bundle, noteService, attachmentService)
	if err != nil {
		t.Fatalf("second ApplyRecovery() error = %v", err)
	}
	if second.AttachmentsUploaded != 0 || second.DocumentsCompleted != 0 || noteService.autosaves != 1 {
		t.Fatalf("second summary = %+v, autosaves = %d", second, noteService.autosaves)
	}
}

func TestApplyRecoveryValidatesEveryNoteBeforeUploading(t *testing.T) {
	bundle := writeRecoveryTestBundle(t, []RecoveryDocument{
		{
			Key: "first", NoteID: "first-id", Title: "First", Confidence: RecoveryConfidenceHigh,
			Mappings: []RecoveryMapping{{MissingReference: "assets/first.png", File: "files/first.png", SHA256Hex: recoverySHA256(recoveryTestPNG)}},
		},
		{
			Key: "second", NoteID: "second-id", Title: "Second", Confidence: RecoveryConfidenceHigh,
			Mappings: []RecoveryMapping{{MissingReference: "assets/missing.png", File: "files/second.png", SHA256Hex: recoverySHA256(recoveryTestPNG)}},
		},
	}, map[string][]byte{"files/first.png": recoveryTestPNG, "files/second.png": recoveryTestPNG})
	first, _ := notes.NewNote("first-id", "space-id", "", "First", "![](assets/first.png)")
	second, _ := notes.NewNote("second-id", "space-id", "", "Second", "the reference was edited")
	noteService := &recoveryNotesFake{notes: map[string]*notes.Note{"first-id": first, "second-id": second}}
	attachmentService := &recoveryAttachmentsFake{}

	_, err := ApplyRecovery(context.Background(), bundle, noteService, attachmentService)
	if err == nil || !strings.Contains(err.Error(), "assets/missing.png") {
		t.Fatalf("ApplyRecovery() error = %v", err)
	}
	if attachmentService.uploads != 0 || noteService.autosaves != 0 {
		t.Fatalf("uploads = %d, autosaves = %d; validation must happen before writes", attachmentService.uploads, noteService.autosaves)
	}
}

func TestApplyRecoveryRecoversUploadMissingFromState(t *testing.T) {
	bundle := writeRecoveryTestBundle(t, []RecoveryDocument{{
		Key: "note-key", NoteID: "note-id", Title: "Note", Confidence: RecoveryConfidenceHigh,
		Mappings: []RecoveryMapping{{MissingReference: "assets/recovered.png", File: "files/recovered.png", SHA256Hex: recoverySHA256(recoveryTestPNG)}},
	}}, map[string][]byte{"files/recovered.png": recoveryTestPNG})
	note, _ := notes.NewNote("note-id", "space-id", "", "Note", "![](assets/recovered.png)")
	noteService := &recoveryNotesFake{notes: map[string]*notes.Note{"note-id": note}}
	attachmentService := &recoveryAttachmentsFake{items: []attachments.Attachment{{
		ID: "existing-id", NoteID: "note-id", OriginalName: "recovered.png", SHA256Hex: recoverySHA256(recoveryTestPNG),
	}}}

	summary, err := ApplyRecovery(context.Background(), bundle, noteService, attachmentService)
	if err != nil {
		t.Fatalf("ApplyRecovery() error = %v", err)
	}
	if summary.AttachmentsUploaded != 0 || note.Markdown() != "![](attachment:existing-id)" {
		t.Fatalf("summary = %+v, Markdown = %q", summary, note.Markdown())
	}
}

func TestValidateRecoveryBundleRejectsNonImageContent(t *testing.T) {
	content := []byte("not an image")
	bundle := writeRecoveryTestBundle(t, []RecoveryDocument{{
		Key: "note-key", NoteID: "note-id", Title: "Note", Confidence: RecoveryConfidenceHigh,
		Mappings: []RecoveryMapping{{MissingReference: "assets/image.png", File: "files/image.png", SHA256Hex: recoverySHA256(content)}},
	}}, map[string][]byte{"files/image.png": content})

	_, err := ValidateRecoveryBundle(bundle)
	if err == nil || !strings.Contains(err.Error(), "supported image") {
		t.Fatalf("ValidateRecoveryBundle() error = %v", err)
	}
}

func writeRecoveryTestBundle(t *testing.T, documents []RecoveryDocument, files map[string][]byte) string {
	t.Helper()
	bundle := t.TempDir()
	manifest, err := json.Marshal(RecoveryManifest{Version: RecoveryManifestVersion, Documents: documents})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		filename := filepath.Join(bundle, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return bundle
}

func recoverySHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

type recoveryNotesFake struct {
	notes     map[string]*notes.Note
	autosaves int
}

func (fake *recoveryNotesFake) GetNote(_ context.Context, id string) (*notes.Note, error) {
	return fake.notes[id], nil
}

func (fake *recoveryNotesFake) Autosave(_ context.Context, id string, expectedVersion int64, title, markdown string) (*notes.Note, error) {
	note := fake.notes[id]
	if err := note.Autosave(expectedVersion, title, markdown); err != nil {
		return nil, err
	}
	fake.autosaves++
	return note, nil
}

type recoveryAttachmentsFake struct {
	items   []attachments.Attachment
	uploads int
}

func (fake *recoveryAttachmentsFake) Upload(_ context.Context, noteID, originalName string, reader io.Reader) (attachments.Attachment, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return attachments.Attachment{}, err
	}
	fake.uploads++
	attachment := attachments.Attachment{
		ID:           "attachment-" + string(rune('0'+fake.uploads)),
		NoteID:       noteID,
		OriginalName: originalName,
		SHA256Hex:    recoverySHA256(content),
		SizeBytes:    int64(len(content)),
		CreatedAt:    time.Now(),
	}
	fake.items = append(fake.items, attachment)
	return attachment, nil
}

func (fake *recoveryAttachmentsFake) ListForNote(_ context.Context, noteID string) ([]attachments.Attachment, error) {
	result := make([]attachments.Attachment, 0)
	for _, attachment := range fake.items {
		if attachment.NoteID == noteID {
			result = append(result, attachment)
		}
	}
	return result, nil
}
