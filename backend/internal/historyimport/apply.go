package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Ken-Chy129/note-foundry/backend/internal/attachments"
	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
)

const (
	applyStateVersion       = 1
	maxApplyManifestBytes   = 16 << 20
	maxApplyAttachmentBytes = 25 << 20
)

var stagedAssetPattern = regexp.MustCompile(`asset:([A-Za-z0-9_-]+)`)

type ValidatedStaging struct {
	Manifest         StageManifest
	StagingSHA256Hex string
	Markdown         map[string]string
	AttachmentCount  int
}

type ApplySummary struct {
	DocumentsPlanned    int
	AttachmentsPlanned  int
	SpacesCreated       int
	DirectoriesCreated  int
	NotesCreated        int
	AttachmentsUploaded int
	DocumentsCompleted  int
	StatePath           string
}

type ApplyKnowledge interface {
	CreateSpace(context.Context, string, knowledge.Visibility) (*knowledge.Space, error)
	ListSpaces(context.Context, int, int) (knowledge.SpacePage, error)
	GetSpace(context.Context, string) (*knowledge.Space, error)
	CreateDirectory(context.Context, string, string, string) (*knowledge.Directory, error)
	GetDirectory(context.Context, string) (*knowledge.Directory, error)
	ListDirectories(context.Context, string) ([]*knowledge.Directory, error)
}

type ApplyNotes interface {
	CreateNote(context.Context, string, string, string, string) (*notes.Note, error)
	GetNote(context.Context, string) (*notes.Note, error)
	Autosave(context.Context, string, int64, string, string) (*notes.Note, error)
	ListNotes(context.Context, notes.NoteListFilter) (notes.NotePage, error)
}

type ApplyAttachments interface {
	Upload(context.Context, string, string, io.Reader) (attachments.Attachment, error)
	ListForNote(context.Context, string) ([]attachments.Attachment, error)
}

type applyState struct {
	Version          int                           `json:"version"`
	StagingSHA256Hex string                        `json:"stagingSha256Hex"`
	SpaceID          string                        `json:"spaceId,omitempty"`
	Directories      map[string]string             `json:"directories"`
	Documents        map[string]applyDocumentState `json:"documents"`
}

type applyDocumentState struct {
	NoteID      string            `json:"noteId"`
	Attachments map[string]string `json:"attachments"`
	Complete    bool              `json:"complete"`
}

func ValidateStaging(stagingDirectory string) (ValidatedStaging, error) {
	manifestContent, err := readStagingFile(stagingDirectory, "manifest.json", maxApplyManifestBytes)
	if err != nil {
		return ValidatedStaging{}, err
	}
	var manifest StageManifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		return ValidatedStaging{}, fmt.Errorf("decode staging manifest: %w", err)
	}
	if manifest.Version != stageManifestVersion {
		return ValidatedStaging{}, fmt.Errorf("unsupported staging manifest version %d", manifest.Version)
	}
	if strings.TrimSpace(manifest.Space.Name) == "" || manifest.Space.Visibility != string(knowledge.VisibilityPrivate) {
		return ValidatedStaging{}, errors.New("staging target must be a named private Knowledge Space")
	}
	if len(manifest.Documents) == 0 {
		return ValidatedStaging{}, errors.New("staging manifest contains no documents")
	}

	validated := ValidatedStaging{Manifest: manifest, Markdown: make(map[string]string, len(manifest.Documents))}
	stagingDigest := sha256.New()
	_, _ = stagingDigest.Write(manifestContent)
	seenDocumentKeys := make(map[string]bool)
	seenMarkdownPaths := make(map[string]bool)
	seenAttachmentKeys := make(map[string]bool)
	for _, document := range manifest.Documents {
		if strings.TrimSpace(document.Key) == "" || seenDocumentKeys[document.Key] {
			return ValidatedStaging{}, fmt.Errorf("document key %q is empty or duplicated", document.Key)
		}
		seenDocumentKeys[document.Key] = true
		if strings.TrimSpace(document.Title) == "" {
			return ValidatedStaging{}, fmt.Errorf("document %q has no title", document.Key)
		}
		if document.MarkdownPath == "" || seenMarkdownPaths[document.MarkdownPath] {
			return ValidatedStaging{}, fmt.Errorf("document %q has an empty or duplicated Markdown path", document.Key)
		}
		seenMarkdownPaths[document.MarkdownPath] = true
		for _, component := range document.Directory {
			if strings.TrimSpace(component) == "" {
				return ValidatedStaging{}, fmt.Errorf("document %q has an empty directory component", document.Key)
			}
		}

		markdownContent, err := readStagingFile(stagingDirectory, document.MarkdownPath, maxDocumentBytes)
		if err != nil {
			return ValidatedStaging{}, fmt.Errorf("validate document %q: %w", document.Key, err)
		}
		markdown := string(markdownContent)
		validated.Markdown[document.Key] = markdown
		_, _ = stagingDigest.Write([]byte("\x00document\x00" + document.Key + "\x00"))
		_, _ = stagingDigest.Write(markdownContent)
		declared := make(map[string]bool, len(document.Attachments))
		for _, attachment := range document.Attachments {
			if strings.TrimSpace(attachment.Key) == "" || seenAttachmentKeys[attachment.Key] {
				return ValidatedStaging{}, fmt.Errorf("attachment key %q is empty or duplicated", attachment.Key)
			}
			seenAttachmentKeys[attachment.Key] = true
			declared[attachment.Key] = true
			if strings.TrimSpace(attachment.Name) == "" || strings.TrimSpace(attachment.Path) == "" {
				return ValidatedStaging{}, fmt.Errorf("attachment %q has no name or path", attachment.Key)
			}
			content, err := readStagingFile(stagingDirectory, attachment.Path, maxApplyAttachmentBytes)
			if err != nil {
				return ValidatedStaging{}, fmt.Errorf("validate attachment %q: %w", attachment.Key, err)
			}
			digest := sha256.Sum256(content)
			if hex.EncodeToString(digest[:]) != strings.ToLower(attachment.SHA256Hex) {
				return ValidatedStaging{}, fmt.Errorf("attachment %q checksum does not match the manifest", attachment.Key)
			}
			if !strings.Contains(markdown, "asset:"+attachment.Key) {
				return ValidatedStaging{}, fmt.Errorf("attachment %q is not referenced by document %q", attachment.Key, document.Key)
			}
			validated.AttachmentCount++
		}
		for _, match := range stagedAssetPattern.FindAllStringSubmatch(markdown, -1) {
			if !declared[match[1]] {
				return ValidatedStaging{}, fmt.Errorf("document %q references undeclared staged asset %q", document.Key, match[1])
			}
		}
	}
	validated.StagingSHA256Hex = hex.EncodeToString(stagingDigest.Sum(nil))
	return validated, nil
}

func ApplyStaging(ctx context.Context, stagingDirectory string, knowledgeService ApplyKnowledge, notesService ApplyNotes, attachmentService ApplyAttachments) (ApplySummary, error) {
	validated, err := ValidateStaging(stagingDirectory)
	if err != nil {
		return ApplySummary{}, err
	}
	statePath := filepath.Join(stagingDirectory, "apply-state.json")
	state, stateExists, err := loadApplyState(statePath, validated.StagingSHA256Hex)
	if err != nil {
		return ApplySummary{}, err
	}
	summary := ApplySummary{
		DocumentsPlanned:   len(validated.Manifest.Documents),
		AttachmentsPlanned: validated.AttachmentCount,
		StatePath:          statePath,
	}

	if !stateExists {
		spaces, err := knowledgeService.ListSpaces(ctx, 1, 1000)
		if err != nil {
			return summary, fmt.Errorf("list existing Knowledge Spaces: %w", err)
		}
		for _, space := range spaces.Spaces {
			if strings.EqualFold(space.Name(), validated.Manifest.Space.Name) {
				return summary, fmt.Errorf("Knowledge Space %q already exists before this import started", validated.Manifest.Space.Name)
			}
		}
		if err := writeApplyState(statePath, state); err != nil {
			return summary, err
		}
		stateExists = true
	}
	if state.SpaceID == "" {
		spaces, err := knowledgeService.ListSpaces(ctx, 1, 1000)
		if err != nil {
			return summary, fmt.Errorf("list Knowledge Spaces while resuming: %w", err)
		}
		for _, existing := range spaces.Spaces {
			if strings.EqualFold(existing.Name(), validated.Manifest.Space.Name) {
				if existing.Visibility() != knowledge.VisibilityPrivate {
					return summary, fmt.Errorf("existing Knowledge Space %q is not private", existing.Name())
				}
				state.SpaceID = existing.ID()
				if err := writeApplyState(statePath, state); err != nil {
					return summary, err
				}
				break
			}
		}
	}
	if state.SpaceID == "" {
		space, err := knowledgeService.CreateSpace(ctx, validated.Manifest.Space.Name, knowledge.VisibilityPrivate)
		if err != nil {
			return summary, fmt.Errorf("create target Knowledge Space: %w", err)
		}
		state.SpaceID = space.ID()
		summary.SpacesCreated++
		if err := writeApplyState(statePath, state); err != nil {
			return summary, err
		}
	} else {
		space, err := knowledgeService.GetSpace(ctx, state.SpaceID)
		if err != nil {
			return summary, fmt.Errorf("load target Knowledge Space from apply state: %w", err)
		}
		if space == nil || space.Name() != validated.Manifest.Space.Name || space.Visibility() != knowledge.VisibilityPrivate {
			return summary, errors.New("target Knowledge Space does not match the apply state")
		}
	}

	directoryPaths := collectDirectoryPaths(validated.Manifest.Documents)
	existingDirectories, err := knowledgeService.ListDirectories(ctx, state.SpaceID)
	if err != nil {
		return summary, fmt.Errorf("list existing import directories: %w", err)
	}
	for _, directoryPath := range directoryPaths {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		key := directoryStateKey(directoryPath)
		parentID := ""
		if len(directoryPath) > 1 {
			parentID = state.Directories[directoryStateKey(directoryPath[:len(directoryPath)-1])]
		}
		if id := state.Directories[key]; id != "" {
			directory, err := knowledgeService.GetDirectory(ctx, id)
			if err != nil {
				return summary, fmt.Errorf("load directory %q from apply state: %w", key, err)
			}
			if directory == nil || directory.SpaceID() != state.SpaceID || directory.ParentID() != parentID || directory.Name() != directoryPath[len(directoryPath)-1] {
				return summary, fmt.Errorf("directory %q does not match the apply state", key)
			}
			continue
		}
		if recovered := recoverRuntimeDirectory(existingDirectories, state.SpaceID, parentID, directoryPath[len(directoryPath)-1]); recovered != "" {
			state.Directories[key] = recovered
			if err := writeApplyState(statePath, state); err != nil {
				return summary, err
			}
			continue
		}
		directory, err := knowledgeService.CreateDirectory(ctx, state.SpaceID, parentID, directoryPath[len(directoryPath)-1])
		if err != nil {
			return summary, fmt.Errorf("create directory %q: %w", key, err)
		}
		state.Directories[key] = directory.ID()
		existingDirectories = append(existingDirectories, directory)
		summary.DirectoriesCreated++
		if err := writeApplyState(statePath, state); err != nil {
			return summary, err
		}
	}

	for _, document := range validated.Manifest.Documents {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		documentState := state.Documents[document.Key]
		if documentState.Attachments == nil {
			documentState.Attachments = make(map[string]string)
		}
		directoryID := ""
		if len(document.Directory) > 0 {
			directoryID = state.Directories[directoryStateKey(document.Directory)]
		}
		initialMarkdown := validated.Markdown[document.Key]
		if documentState.NoteID == "" {
			existingNotes, err := notesService.ListNotes(ctx, notes.NoteListFilter{SpaceID: state.SpaceID, DirectoryID: directoryID, Page: 1, PageSize: 1000})
			if err != nil {
				return summary, fmt.Errorf("list existing notes while resuming %q: %w", document.Title, err)
			}
			if recovered := recoverRuntimeNote(existingNotes.Notes, directoryID, document.Title, initialMarkdown); recovered != "" {
				documentState.NoteID = recovered
				state.Documents[document.Key] = documentState
				if err := writeApplyState(statePath, state); err != nil {
					return summary, err
				}
			}
		}
		if documentState.NoteID == "" {
			note, err := notesService.CreateNote(ctx, state.SpaceID, directoryID, document.Title, initialMarkdown)
			if err != nil {
				return summary, fmt.Errorf("create document %q: %w", document.Title, err)
			}
			documentState.NoteID = note.ID()
			state.Documents[document.Key] = documentState
			summary.NotesCreated++
			if err := writeApplyState(statePath, state); err != nil {
				return summary, err
			}
		}

		note, err := notesService.GetNote(ctx, documentState.NoteID)
		if err != nil {
			return summary, fmt.Errorf("load document %q from apply state: %w", document.Title, err)
		}
		if note == nil || note.SpaceID() != state.SpaceID || note.DirectoryID() != directoryID || note.Title() != document.Title {
			return summary, fmt.Errorf("document %q does not match the apply state", document.Title)
		}

		existingAttachments, err := attachmentService.ListForNote(ctx, documentState.NoteID)
		if err != nil {
			return summary, fmt.Errorf("list attachments for document %q: %w", document.Title, err)
		}
		for _, staged := range document.Attachments {
			if attachmentID := documentState.Attachments[staged.Key]; attachmentID != "" {
				if !matchingRuntimeAttachment(existingAttachments, attachmentID, staged) {
					return summary, fmt.Errorf("attachment %q does not match the apply state", staged.Key)
				}
				continue
			}
			if recovered := recoverRuntimeAttachment(existingAttachments, staged); recovered != "" {
				documentState.Attachments[staged.Key] = recovered
				state.Documents[document.Key] = documentState
				if err := writeApplyState(statePath, state); err != nil {
					return summary, err
				}
				continue
			}
			filename, err := stagingFilePath(stagingDirectory, staged.Path)
			if err != nil {
				return summary, err
			}
			file, err := os.Open(filename)
			if err != nil {
				return summary, fmt.Errorf("open attachment %q: %w", staged.Key, err)
			}
			uploaded, uploadErr := attachmentService.Upload(ctx, documentState.NoteID, staged.Name, file)
			closeErr := file.Close()
			if uploadErr != nil {
				return summary, fmt.Errorf("upload attachment %q: %w", staged.Key, uploadErr)
			}
			if closeErr != nil {
				return summary, fmt.Errorf("close attachment %q: %w", staged.Key, closeErr)
			}
			if uploaded.SHA256Hex != staged.SHA256Hex {
				return summary, fmt.Errorf("uploaded attachment %q checksum mismatch", staged.Key)
			}
			documentState.Attachments[staged.Key] = uploaded.ID
			existingAttachments = append(existingAttachments, uploaded)
			state.Documents[document.Key] = documentState
			summary.AttachmentsUploaded++
			if err := writeApplyState(statePath, state); err != nil {
				return summary, err
			}
		}

		finalMarkdown := initialMarkdown
		for stagedKey, attachmentID := range documentState.Attachments {
			finalMarkdown = strings.ReplaceAll(finalMarkdown, "asset:"+stagedKey, "attachment:"+attachmentID)
		}
		if stagedAssetPattern.MatchString(finalMarkdown) {
			return summary, fmt.Errorf("document %q still contains unresolved staged assets", document.Title)
		}
		note, err = notesService.GetNote(ctx, documentState.NoteID)
		if err != nil {
			return summary, fmt.Errorf("reload document %q: %w", document.Title, err)
		}
		if note.Markdown() != finalMarkdown {
			if _, err := notesService.Autosave(ctx, note.ID(), note.Version(), document.Title, finalMarkdown); err != nil {
				return summary, fmt.Errorf("save final Markdown for document %q: %w", document.Title, err)
			}
		}
		if !documentState.Complete {
			documentState.Complete = true
			state.Documents[document.Key] = documentState
			summary.DocumentsCompleted++
			if err := writeApplyState(statePath, state); err != nil {
				return summary, err
			}
		}
	}
	return summary, nil
}

func collectDirectoryPaths(documents []StageDocument) [][]string {
	unique := make(map[string][]string)
	for _, document := range documents {
		for depth := 1; depth <= len(document.Directory); depth++ {
			parts := append([]string(nil), document.Directory[:depth]...)
			unique[directoryStateKey(parts)] = parts
		}
	}
	paths := make([][]string, 0, len(unique))
	for _, parts := range unique {
		paths = append(paths, parts)
	}
	sort.Slice(paths, func(left, right int) bool {
		if len(paths[left]) != len(paths[right]) {
			return len(paths[left]) < len(paths[right])
		}
		return directoryStateKey(paths[left]) < directoryStateKey(paths[right])
	})
	return paths
}

func directoryStateKey(parts []string) string {
	encoded, _ := json.Marshal(parts)
	return string(encoded)
}

func recoverRuntimeDirectory(existing []*knowledge.Directory, spaceID, parentID, name string) string {
	for _, directory := range existing {
		if directory.SpaceID() == spaceID && directory.ParentID() == parentID && directory.Name() == name {
			return directory.ID()
		}
	}
	return ""
}

func recoverRuntimeNote(existing []*notes.Note, directoryID, title, markdown string) string {
	match := ""
	for _, note := range existing {
		if note.DirectoryID() == directoryID && note.Title() == title && note.Markdown() == markdown {
			if match != "" {
				return ""
			}
			match = note.ID()
		}
	}
	return match
}

func matchingRuntimeAttachment(existing []attachments.Attachment, id string, staged StageAttachment) bool {
	for _, attachment := range existing {
		if attachment.ID == id {
			return attachment.NoteID != "" && attachment.OriginalName == staged.Name && attachment.SHA256Hex == staged.SHA256Hex
		}
	}
	return false
}

func recoverRuntimeAttachment(existing []attachments.Attachment, staged StageAttachment) string {
	match := ""
	for _, attachment := range existing {
		if attachment.OriginalName == staged.Name && attachment.SHA256Hex == staged.SHA256Hex {
			if match != "" {
				return ""
			}
			match = attachment.ID
		}
	}
	return match
}

func loadApplyState(filename, stagingSHA256Hex string) (applyState, bool, error) {
	state := applyState{
		Version:          applyStateVersion,
		StagingSHA256Hex: stagingSHA256Hex,
		Directories:      make(map[string]string),
		Documents:        make(map[string]applyDocumentState),
	}
	content, err := readFileLimited(filename, maxApplyManifestBytes)
	if errors.Is(err, os.ErrNotExist) {
		return state, false, nil
	}
	if err != nil {
		return applyState{}, false, fmt.Errorf("read apply state: %w", err)
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return applyState{}, false, fmt.Errorf("decode apply state: %w", err)
	}
	if state.Version != applyStateVersion || state.StagingSHA256Hex != stagingSHA256Hex {
		return applyState{}, false, errors.New("apply state does not match the staging package")
	}
	if state.Directories == nil {
		state.Directories = make(map[string]string)
	}
	if state.Documents == nil {
		state.Documents = make(map[string]applyDocumentState)
	}
	return state, true, nil
}

func writeApplyState(filename string, state applyState) error {
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode apply state: %w", err)
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return fmt.Errorf("create apply state directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".apply-state-*")
	if err != nil {
		return fmt.Errorf("create temporary apply state: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary apply state: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary apply state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary apply state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary apply state: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("replace apply state: %w", err)
	}
	return nil
}

func readStagingFile(stagingDirectory, relativePath string, limit int64) ([]byte, error) {
	filename, err := stagingFilePath(stagingDirectory, relativePath)
	if err != nil {
		return nil, err
	}
	content, err := readFileLimited(filename, limit)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func stagingFilePath(stagingDirectory, relativePath string) (string, error) {
	root, err := filepath.EvalSymlinks(stagingDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve staging directory: %w", err)
	}
	candidate, err := safeStagePath(root, relativePath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve staging file %s: %w", relativePath, err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("staging file %s escapes the staging directory", relativePath)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect staging file %s: %w", relativePath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("staging file %s is not a regular file", relativePath)
	}
	return resolved, nil
}
