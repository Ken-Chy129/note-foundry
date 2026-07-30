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
	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
)

func TestValidateStagingChecksAttachmentHashes(t *testing.T) {
	staging := writeApplyTestStaging(t)
	validated, err := ValidateStaging(staging)
	if err != nil {
		t.Fatalf("ValidateStaging() error = %v", err)
	}
	if len(validated.Manifest.Documents) != 1 || validated.AttachmentCount != 1 {
		t.Fatalf("ValidateStaging() = %+v", validated)
	}

	if err := os.WriteFile(filepath.Join(staging, "attachments", "note", "image.png"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateStaging(staging); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("ValidateStaging() error = %v, want checksum failure", err)
	}
}

func TestApplyStagingUsesDomainServicesAndResumesWithoutDuplicates(t *testing.T) {
	staging := writeApplyTestStaging(t)
	runtime := newApplyRuntimeFake()

	summary, err := ApplyStaging(context.Background(), staging, runtime, runtime, runtime)
	if err != nil {
		t.Fatalf("ApplyStaging() error = %v", err)
	}
	if summary.SpacesCreated != 1 || summary.DirectoriesCreated != 2 || summary.NotesCreated != 1 || summary.AttachmentsUploaded != 1 {
		t.Fatalf("ApplyStaging() summary = %+v", summary)
	}
	note := runtime.onlyNote(t)
	if !strings.Contains(note.Markdown(), "attachment:attachment-1") {
		t.Fatalf("note markdown = %q, want managed attachment", note.Markdown())
	}
	if !strings.Contains(note.Markdown(), "assets/missing.png") {
		t.Fatalf("note markdown = %q, missing SiYuan reference was changed", note.Markdown())
	}

	second, err := ApplyStaging(context.Background(), staging, runtime, runtime, runtime)
	if err != nil {
		t.Fatalf("second ApplyStaging() error = %v", err)
	}
	if second.SpacesCreated != 0 || second.DirectoriesCreated != 0 || second.NotesCreated != 0 || second.AttachmentsUploaded != 0 {
		t.Fatalf("second ApplyStaging() summary = %+v, want idempotent resume", second)
	}
	if runtime.spaceCreates != 1 || runtime.directoryCreates != 2 || runtime.noteCreates != 1 || runtime.attachmentUploads != 1 {
		t.Fatalf("runtime create counts = space:%d directory:%d note:%d attachment:%d", runtime.spaceCreates, runtime.directoryCreates, runtime.noteCreates, runtime.attachmentUploads)
	}
}

func TestApplyStagingRejectsChangedMarkdownAfterStateWasCreated(t *testing.T) {
	staging := writeApplyTestStaging(t)
	runtime := newApplyRuntimeFake()
	if _, err := ApplyStaging(context.Background(), staging, runtime, runtime, runtime); err != nil {
		t.Fatalf("ApplyStaging() error = %v", err)
	}
	markdownPath := filepath.Join(staging, "notes", "Java NIO.md")
	if err := os.WriteFile(markdownPath, []byte("# Java NIO\n\nchanged\n\n![已有](asset:asset-key)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyStaging(context.Background(), staging, runtime, runtime, runtime); err == nil || !strings.Contains(err.Error(), "staging package") {
		t.Fatalf("ApplyStaging() error = %v, want changed staging rejection", err)
	}
}

func TestApplyStagingRecoversWritesCompletedBeforeStateFlush(t *testing.T) {
	staging := writeApplyTestStaging(t)
	validated, err := ValidateStaging(staging)
	if err != nil {
		t.Fatal(err)
	}
	state := applyState{
		Version:          applyStateVersion,
		StagingSHA256Hex: validated.StagingSHA256Hex,
		Directories:      make(map[string]string),
		Documents:        make(map[string]applyDocumentState),
	}
	if err := writeApplyState(filepath.Join(staging, "apply-state.json"), state); err != nil {
		t.Fatal(err)
	}

	runtime := newApplyRuntimeFake()
	space, _ := runtime.CreateSpace(context.Background(), "历史文档", knowledge.VisibilityPrivate)
	root, _ := runtime.CreateDirectory(context.Background(), space.ID(), "", "Java 与 JVM")
	child, _ := runtime.CreateDirectory(context.Background(), space.ID(), root.ID(), "Java")
	note, _ := runtime.CreateNote(context.Background(), space.ID(), child.ID(), "Java NIO", validated.Markdown["note-key"])
	attachmentFile, err := os.Open(filepath.Join(staging, "attachments", "note", "image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Upload(context.Background(), note.ID(), "image.png", attachmentFile); err != nil {
		t.Fatal(err)
	}
	if err := attachmentFile.Close(); err != nil {
		t.Fatal(err)
	}
	runtime.spaceCreates = 0
	runtime.directoryCreates = 0
	runtime.noteCreates = 0
	runtime.attachmentUploads = 0

	summary, err := ApplyStaging(context.Background(), staging, runtime, runtime, runtime)
	if err != nil {
		t.Fatalf("ApplyStaging() error = %v", err)
	}
	if summary.SpacesCreated != 0 || summary.DirectoriesCreated != 0 || summary.NotesCreated != 0 || summary.AttachmentsUploaded != 0 || summary.DocumentsCompleted != 1 {
		t.Fatalf("ApplyStaging() summary = %+v", summary)
	}
	if !strings.Contains(runtime.onlyNote(t).Markdown(), "attachment:attachment-1") {
		t.Fatalf("recovered note Markdown = %q", runtime.onlyNote(t).Markdown())
	}
}

func writeApplyTestStaging(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	attachmentContent := []byte("image-data")
	digest := sha256.Sum256(attachmentContent)
	manifest := StageManifest{
		Version: stageManifestVersion,
		Space:   StageSpace{Name: "历史文档", Visibility: "private"},
		Documents: []StageDocument{{
			Key:          "note-key",
			Title:        "Java NIO",
			Directory:    []string{"Java 与 JVM", "Java"},
			MarkdownPath: "notes/Java NIO.md",
			Source:       SourceDescriptor{Kind: SourceSiyuan, Collection: "Java学习", Path: "siyuan.zip!Java NIO.md"},
			Attachments: []StageAttachment{{
				Key:       "asset-key",
				Path:      "attachments/note/image.png",
				Name:      "image.png",
				MediaType: "image/png",
				SHA256Hex: hex.EncodeToString(digest[:]),
				Source:    "assets/image.png",
			}},
		}},
	}
	manifestContent, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"manifest.json":              manifestContent,
		"notes/Java NIO.md":          []byte("# Java NIO\n\n![已有](asset:asset-key)\n\n![待补](assets/missing.png)\n"),
		"attachments/note/image.png": attachmentContent,
	} {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type applyRuntimeFake struct {
	spaces            map[string]*knowledge.Space
	directories       map[string]*knowledge.Directory
	notes             map[string]*notes.Note
	attachments       map[string][]attachments.Attachment
	spaceCreates      int
	directoryCreates  int
	noteCreates       int
	attachmentUploads int
}

func newApplyRuntimeFake() *applyRuntimeFake {
	return &applyRuntimeFake{
		spaces:      make(map[string]*knowledge.Space),
		directories: make(map[string]*knowledge.Directory),
		notes:       make(map[string]*notes.Note),
		attachments: make(map[string][]attachments.Attachment),
	}
}

func (runtime *applyRuntimeFake) CreateSpace(_ context.Context, name string, visibility knowledge.Visibility) (*knowledge.Space, error) {
	runtime.spaceCreates++
	space, err := knowledge.NewSpace("space-1", name, visibility)
	if err == nil {
		runtime.spaces[space.ID()] = space
	}
	return space, err
}

func (runtime *applyRuntimeFake) ListSpaces(context.Context, int, int) (knowledge.SpacePage, error) {
	page := knowledge.SpacePage{}
	for _, space := range runtime.spaces {
		page.Spaces = append(page.Spaces, space)
	}
	page.TotalItems = len(page.Spaces)
	return page, nil
}

func (runtime *applyRuntimeFake) GetSpace(_ context.Context, id string) (*knowledge.Space, error) {
	return runtime.spaces[id], nil
}

func (runtime *applyRuntimeFake) CreateDirectory(_ context.Context, spaceID, parentID, name string) (*knowledge.Directory, error) {
	runtime.directoryCreates++
	id := "directory-" + string(rune('0'+runtime.directoryCreates))
	directory, err := knowledge.NewDirectory(id, spaceID, parentID, name)
	if err == nil {
		runtime.directories[id] = directory
	}
	return directory, err
}

func (runtime *applyRuntimeFake) GetDirectory(_ context.Context, id string) (*knowledge.Directory, error) {
	return runtime.directories[id], nil
}

func (runtime *applyRuntimeFake) ListDirectories(context.Context, string) ([]*knowledge.Directory, error) {
	result := make([]*knowledge.Directory, 0, len(runtime.directories))
	for _, directory := range runtime.directories {
		result = append(result, directory)
	}
	return result, nil
}

func (runtime *applyRuntimeFake) CreateNote(_ context.Context, spaceID, directoryID, title, markdown string) (*notes.Note, error) {
	runtime.noteCreates++
	note, err := notes.NewNote("note-1", spaceID, directoryID, title, markdown)
	if err == nil {
		runtime.notes[note.ID()] = note
	}
	return note, err
}

func (runtime *applyRuntimeFake) GetNote(_ context.Context, id string) (*notes.Note, error) {
	return runtime.notes[id], nil
}

func (runtime *applyRuntimeFake) Autosave(_ context.Context, id string, expectedVersion int64, title, markdown string) (*notes.Note, error) {
	note := runtime.notes[id]
	if err := note.Autosave(expectedVersion, title, markdown); err != nil {
		return nil, err
	}
	return note, nil
}

func (runtime *applyRuntimeFake) ListNotes(_ context.Context, filter notes.NoteListFilter) (notes.NotePage, error) {
	page := notes.NotePage{Page: filter.Page, PageSize: filter.PageSize}
	for _, note := range runtime.notes {
		if filter.SpaceID != "" && note.SpaceID() != filter.SpaceID {
			continue
		}
		if filter.DirectoryID != "" && note.DirectoryID() != filter.DirectoryID {
			continue
		}
		page.Notes = append(page.Notes, note)
	}
	page.TotalItems = len(page.Notes)
	return page, nil
}

func (runtime *applyRuntimeFake) Upload(_ context.Context, noteID, originalName string, reader io.Reader) (attachments.Attachment, error) {
	runtime.attachmentUploads++
	content, err := io.ReadAll(reader)
	if err != nil {
		return attachments.Attachment{}, err
	}
	digest := sha256.Sum256(content)
	attachment := attachments.Attachment{
		ID:           "attachment-1",
		NoteID:       noteID,
		OriginalName: originalName,
		SizeBytes:    int64(len(content)),
		SHA256Hex:    hex.EncodeToString(digest[:]),
		CreatedAt:    time.Now(),
	}
	runtime.attachments[noteID] = append(runtime.attachments[noteID], attachment)
	return attachment, nil
}

func (runtime *applyRuntimeFake) ListForNote(_ context.Context, noteID string) ([]attachments.Attachment, error) {
	return append([]attachments.Attachment(nil), runtime.attachments[noteID]...), nil
}

func (runtime *applyRuntimeFake) onlyNote(t *testing.T) *notes.Note {
	t.Helper()
	if len(runtime.notes) != 1 {
		t.Fatalf("notes = %d, want 1", len(runtime.notes))
	}
	for _, note := range runtime.notes {
		return note
	}
	return nil
}
