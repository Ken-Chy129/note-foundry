package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Ken-Chy129/note-foundry/backend/internal/attachments"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
)

const (
	RecoveryManifestVersion = 1
	RecoveryConfidenceHigh  = "high"

	recoveryStateVersion = 1
)

var supportedRecoveryMediaTypes = map[string]bool{
	"image/gif":  true,
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type RecoveryManifest struct {
	Version   int                `json:"version"`
	Documents []RecoveryDocument `json:"documents"`
}

type RecoveryDocument struct {
	Key        string            `json:"key"`
	NoteID     string            `json:"noteId"`
	Title      string            `json:"title"`
	Confidence string            `json:"confidence"`
	Mappings   []RecoveryMapping `json:"mappings"`
}

type RecoveryMapping struct {
	MissingReference string `json:"missingReference"`
	File             string `json:"file"`
	SHA256Hex        string `json:"sha256"`
	SourceURL        string `json:"sourceUrl,omitempty"`
}

type ValidatedRecovery struct {
	Manifest        RecoveryManifest
	BundleSHA256Hex string
	AttachmentCount int
}

type RecoverySummary struct {
	DocumentsPlanned    int
	AttachmentsPlanned  int
	AttachmentsUploaded int
	DocumentsCompleted  int
	StatePath           string
}

type RecoveryNotes interface {
	GetNote(context.Context, string) (*notes.Note, error)
	Autosave(context.Context, string, int64, string, string) (*notes.Note, error)
}

type RecoveryAttachments interface {
	Upload(context.Context, string, string, io.Reader) (attachments.Attachment, error)
	ListForNote(context.Context, string) ([]attachments.Attachment, error)
}

type recoveryState struct {
	Version         int                              `json:"version"`
	BundleSHA256Hex string                           `json:"bundleSha256Hex"`
	Documents       map[string]recoveryDocumentState `json:"documents"`
}

type recoveryDocumentState struct {
	Attachments map[string]string `json:"attachments"`
	Complete    bool              `json:"complete"`
}

type preparedRecoveryDocument struct {
	document    RecoveryDocument
	note        *notes.Note
	attachments []attachments.Attachment
}

func ValidateRecoveryBundle(bundleDirectory string) (ValidatedRecovery, error) {
	manifestContent, err := readStagingFile(bundleDirectory, "manifest.json", maxApplyManifestBytes)
	if err != nil {
		return ValidatedRecovery{}, fmt.Errorf("read recovery manifest: %w", err)
	}
	var manifest RecoveryManifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		return ValidatedRecovery{}, fmt.Errorf("decode recovery manifest: %w", err)
	}
	if manifest.Version != RecoveryManifestVersion {
		return ValidatedRecovery{}, fmt.Errorf("unsupported recovery manifest version %d", manifest.Version)
	}
	if len(manifest.Documents) == 0 {
		return ValidatedRecovery{}, errors.New("recovery manifest contains no documents")
	}

	digest := sha256.New()
	_, _ = digest.Write(manifestContent)
	seenDocuments := make(map[string]bool)
	seenNoteIDs := make(map[string]bool)
	seenFiles := make(map[string]bool)
	attachmentCount := 0
	for _, document := range manifest.Documents {
		if strings.TrimSpace(document.Key) == "" || seenDocuments[document.Key] {
			return ValidatedRecovery{}, fmt.Errorf("recovery document key %q is empty or duplicated", document.Key)
		}
		seenDocuments[document.Key] = true
		if strings.TrimSpace(document.NoteID) == "" || seenNoteIDs[document.NoteID] {
			return ValidatedRecovery{}, fmt.Errorf("recovery note id %q is empty or duplicated", document.NoteID)
		}
		seenNoteIDs[document.NoteID] = true
		if strings.TrimSpace(document.Title) == "" {
			return ValidatedRecovery{}, fmt.Errorf("recovery document %q has no title", document.Key)
		}
		if document.Confidence != RecoveryConfidenceHigh {
			return ValidatedRecovery{}, fmt.Errorf("recovery document %q is not high confidence", document.Key)
		}
		if len(document.Mappings) == 0 {
			return ValidatedRecovery{}, fmt.Errorf("recovery document %q has no asset mappings", document.Key)
		}
		seenReferences := make(map[string]bool)
		for _, mapping := range document.Mappings {
			if !validMissingAssetReference(mapping.MissingReference) || seenReferences[mapping.MissingReference] {
				return ValidatedRecovery{}, fmt.Errorf("recovery document %q has invalid or duplicated missing reference %q", document.Key, mapping.MissingReference)
			}
			seenReferences[mapping.MissingReference] = true
			if strings.TrimSpace(mapping.File) == "" || seenFiles[mapping.File] {
				return ValidatedRecovery{}, fmt.Errorf("recovery file %q is empty or duplicated", mapping.File)
			}
			seenFiles[mapping.File] = true
			content, err := readStagingFile(bundleDirectory, mapping.File, maxApplyAttachmentBytes)
			if err != nil {
				return ValidatedRecovery{}, fmt.Errorf("read recovered asset %q: %w", mapping.File, err)
			}
			checksum := sha256.Sum256(content)
			actualSHA256Hex := hex.EncodeToString(checksum[:])
			if actualSHA256Hex != strings.ToLower(strings.TrimSpace(mapping.SHA256Hex)) {
				return ValidatedRecovery{}, fmt.Errorf("recovered asset %q checksum does not match the manifest", mapping.File)
			}
			mediaType := http.DetectContentType(content)
			if !supportedRecoveryMediaTypes[mediaType] {
				return ValidatedRecovery{}, fmt.Errorf("recovered asset %q is not a supported image: %s", mapping.File, mediaType)
			}
			_, _ = digest.Write([]byte("\x00asset\x00" + document.Key + "\x00" + mapping.MissingReference + "\x00"))
			_, _ = digest.Write(content)
			attachmentCount++
		}
	}
	return ValidatedRecovery{
		Manifest:        manifest,
		BundleSHA256Hex: hex.EncodeToString(digest.Sum(nil)),
		AttachmentCount: attachmentCount,
	}, nil
}

func ApplyRecovery(ctx context.Context, bundleDirectory string, notesService RecoveryNotes, attachmentService RecoveryAttachments) (RecoverySummary, error) {
	validated, err := ValidateRecoveryBundle(bundleDirectory)
	if err != nil {
		return RecoverySummary{}, err
	}
	statePath := filepath.Join(bundleDirectory, "recovery-state.json")
	state, err := loadRecoveryState(statePath, validated.BundleSHA256Hex)
	if err != nil {
		return RecoverySummary{}, err
	}
	summary := RecoverySummary{
		DocumentsPlanned:   len(validated.Manifest.Documents),
		AttachmentsPlanned: validated.AttachmentCount,
		StatePath:          statePath,
	}

	prepared := make([]preparedRecoveryDocument, 0, len(validated.Manifest.Documents))
	for _, document := range validated.Manifest.Documents {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		note, err := notesService.GetNote(ctx, document.NoteID)
		if err != nil {
			return summary, fmt.Errorf("load recovery document %q: %w", document.Title, err)
		}
		if note == nil || note.ID() != document.NoteID || note.Title() != document.Title {
			return summary, fmt.Errorf("recovery document %q does not match the current Learning Note", document.Title)
		}
		existing, err := attachmentService.ListForNote(ctx, document.NoteID)
		if err != nil {
			return summary, fmt.Errorf("list attachments for recovery document %q: %w", document.Title, err)
		}
		documentState := state.Documents[document.Key]
		if documentState.Attachments == nil {
			documentState.Attachments = make(map[string]string)
		}
		for _, mapping := range document.Mappings {
			attachmentID := documentState.Attachments[mapping.MissingReference]
			if attachmentID != "" {
				if !matchingRecoveryAttachment(existing, attachmentID, document.NoteID, mapping) {
					return summary, fmt.Errorf("recovered asset %q does not match recovery state", mapping.MissingReference)
				}
			} else {
				recovered, err := recoverExistingAttachment(existing, document.NoteID, mapping)
				if err != nil {
					return summary, err
				}
				if recovered != "" {
					documentState.Attachments[mapping.MissingReference] = recovered
					attachmentID = recovered
				}
			}
			if err := validateRecoveryReference(note.Markdown(), mapping.MissingReference, attachmentID); err != nil {
				return summary, fmt.Errorf("validate recovery document %q: %w", document.Title, err)
			}
		}
		if documentState.Complete {
			for _, mapping := range document.Mappings {
				if documentState.Attachments[mapping.MissingReference] == "" {
					return summary, fmt.Errorf("completed recovery document %q has unresolved state", document.Title)
				}
			}
		}
		state.Documents[document.Key] = documentState
		prepared = append(prepared, preparedRecoveryDocument{document: document, note: note, attachments: existing})
	}
	if err := writeRecoveryState(statePath, state); err != nil {
		return summary, err
	}

	for _, preparedDocument := range prepared {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		document := preparedDocument.document
		documentState := state.Documents[document.Key]
		for _, mapping := range document.Mappings {
			if documentState.Attachments[mapping.MissingReference] != "" {
				continue
			}
			filename, err := stagingFilePath(bundleDirectory, mapping.File)
			if err != nil {
				return summary, err
			}
			file, err := os.Open(filename)
			if err != nil {
				return summary, fmt.Errorf("open recovered asset %q: %w", mapping.File, err)
			}
			uploaded, uploadErr := attachmentService.Upload(ctx, document.NoteID, path.Base(mapping.MissingReference), file)
			closeErr := file.Close()
			if uploadErr != nil {
				return summary, fmt.Errorf("upload recovered asset %q: %w", mapping.MissingReference, uploadErr)
			}
			if closeErr != nil {
				return summary, fmt.Errorf("close recovered asset %q: %w", mapping.MissingReference, closeErr)
			}
			if uploaded.ID == "" || uploaded.NoteID != document.NoteID || uploaded.OriginalName != path.Base(mapping.MissingReference) || uploaded.SHA256Hex != strings.ToLower(mapping.SHA256Hex) {
				return summary, fmt.Errorf("uploaded recovered asset %q does not match the manifest", mapping.MissingReference)
			}
			documentState.Attachments[mapping.MissingReference] = uploaded.ID
			state.Documents[document.Key] = documentState
			summary.AttachmentsUploaded++
			if err := writeRecoveryState(statePath, state); err != nil {
				return summary, err
			}
		}

		note, err := notesService.GetNote(ctx, document.NoteID)
		if err != nil {
			return summary, fmt.Errorf("reload recovery document %q: %w", document.Title, err)
		}
		if note == nil || note.Title() != document.Title {
			return summary, fmt.Errorf("recovery document %q changed while assets were uploaded", document.Title)
		}
		finalMarkdown := note.Markdown()
		for _, mapping := range document.Mappings {
			attachmentID := documentState.Attachments[mapping.MissingReference]
			if err := validateRecoveryReference(finalMarkdown, mapping.MissingReference, attachmentID); err != nil {
				return summary, fmt.Errorf("finalize recovery document %q: %w", document.Title, err)
			}
			finalMarkdown = strings.Replace(finalMarkdown, mapping.MissingReference, "attachment:"+attachmentID, 1)
		}
		if note.Markdown() != finalMarkdown {
			if _, err := notesService.Autosave(ctx, note.ID(), note.Version(), note.Title(), finalMarkdown); err != nil {
				return summary, fmt.Errorf("save recovered Markdown for document %q: %w", document.Title, err)
			}
		}
		if !documentState.Complete {
			documentState.Complete = true
			state.Documents[document.Key] = documentState
			summary.DocumentsCompleted++
			if err := writeRecoveryState(statePath, state); err != nil {
				return summary, err
			}
		}
	}
	return summary, nil
}

func validMissingAssetReference(reference string) bool {
	if reference == "" || strings.Contains(reference, "\\") || !strings.HasPrefix(reference, "assets/") {
		return false
	}
	cleaned := path.Clean(reference)
	return cleaned == reference && cleaned != "assets" && !strings.Contains(reference, "#") && !strings.Contains(reference, "?")
}

func validateRecoveryReference(markdown, missingReference, attachmentID string) error {
	missingCount := strings.Count(markdown, missingReference)
	if attachmentID == "" {
		if missingCount != 1 {
			return fmt.Errorf("missing reference %q occurs %d times; want exactly once", missingReference, missingCount)
		}
		return nil
	}
	attachmentReference := "attachment:" + attachmentID
	attachmentCount := strings.Count(markdown, attachmentReference)
	if (missingCount == 1 && attachmentCount == 0) || (missingCount == 0 && attachmentCount == 1) {
		return nil
	}
	return fmt.Errorf("reference %q has missing count %d and recovered count %d", missingReference, missingCount, attachmentCount)
}

func matchingRecoveryAttachment(existing []attachments.Attachment, id, noteID string, mapping RecoveryMapping) bool {
	for _, attachment := range existing {
		if attachment.ID == id {
			return attachment.NoteID == noteID && attachment.OriginalName == path.Base(mapping.MissingReference) && attachment.SHA256Hex == strings.ToLower(mapping.SHA256Hex)
		}
	}
	return false
}

func recoverExistingAttachment(existing []attachments.Attachment, noteID string, mapping RecoveryMapping) (string, error) {
	match := ""
	for _, attachment := range existing {
		if attachment.NoteID == noteID && attachment.OriginalName == path.Base(mapping.MissingReference) && attachment.SHA256Hex == strings.ToLower(mapping.SHA256Hex) {
			if match != "" {
				return "", fmt.Errorf("multiple existing attachments match recovered asset %q", mapping.MissingReference)
			}
			match = attachment.ID
		}
	}
	return match, nil
}

func loadRecoveryState(filename, bundleSHA256Hex string) (recoveryState, error) {
	state := recoveryState{
		Version:         recoveryStateVersion,
		BundleSHA256Hex: bundleSHA256Hex,
		Documents:       make(map[string]recoveryDocumentState),
	}
	content, err := readFileLimited(filename, maxApplyManifestBytes)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return recoveryState{}, fmt.Errorf("read recovery state: %w", err)
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return recoveryState{}, fmt.Errorf("decode recovery state: %w", err)
	}
	if state.Version != recoveryStateVersion || state.BundleSHA256Hex != bundleSHA256Hex {
		return recoveryState{}, errors.New("recovery state does not match the recovery bundle")
	}
	if state.Documents == nil {
		state.Documents = make(map[string]recoveryDocumentState)
	}
	return state, nil
}

func writeRecoveryState(filename string, state recoveryState) error {
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode recovery state: %w", err)
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return fmt.Errorf("create recovery state directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".recovery-state-*")
	if err != nil {
		return fmt.Errorf("create temporary recovery state: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary recovery state: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary recovery state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary recovery state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary recovery state: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("replace recovery state: %w", err)
	}
	return nil
}
