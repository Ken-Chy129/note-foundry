package notes

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

var (
	ErrNoteIDRequired                    = errors.New("Learning Note id is required")
	ErrNoteSpaceIDRequired               = errors.New("Learning Note Knowledge Space id is required")
	ErrNoteTitleRequired                 = errors.New("Learning Note title is required")
	ErrRevisionIDRequired                = errors.New("Note Revision id is required")
	ErrRevisionWrongNote                 = errors.New("Note Revision belongs to another Learning Note")
	ErrVersionConflict                   = errors.New("Learning Note was changed by another save")
	ErrPublicRestoreConfirmationRequired = errors.New("restoring a Learning Note to a public Knowledge Space requires publish confirmation")
	ErrPublicMoveConfirmationRequired    = errors.New("moving a private Learning Note to a public Knowledge Space requires publish confirmation")
)

type RevisionReason string

const (
	RevisionReasonPublish RevisionReason = "publish"
	RevisionReasonRestore RevisionReason = "restore"
	RevisionReasonManual  RevisionReason = "manual"
)

type PublishedContent struct {
	Title       string
	Slug        string
	Markdown    string
	PublishedAt time.Time
}

type Revision struct {
	ID        string
	NoteID    string
	Title     string
	Slug      string
	Markdown  string
	Reason    RevisionReason
	CreatedAt time.Time
}

type Note struct {
	id          string
	spaceID     string
	directoryID string
	title       string
	slug        string
	markdown    string
	version     int64
	published   *PublishedContent
	updatedAt   time.Time
}

func NewNote(id, spaceID, directoryID, title, markdown string) (*Note, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNoteIDRequired
	}
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return nil, ErrNoteSpaceIDRequired
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrNoteTitleRequired
	}
	return &Note{
		id:          id,
		spaceID:     spaceID,
		directoryID: strings.TrimSpace(directoryID),
		title:       title,
		slug:        slugify(title),
		markdown:    markdown,
		version:     1,
	}, nil
}

func (note *Note) ID() string {
	return note.id
}

func (note *Note) SpaceID() string {
	return note.spaceID
}

func (note *Note) DirectoryID() string {
	return note.directoryID
}

func (note *Note) Title() string {
	return note.title
}

func (note *Note) Slug() string {
	return note.slug
}

func (note *Note) Markdown() string {
	return note.markdown
}

func (note *Note) Version() int64 {
	return note.version
}

func (note *Note) Published() *PublishedContent {
	return note.published
}

func (note *Note) UpdatedAt() time.Time {
	return note.updatedAt
}

func (note *Note) markUpdatedAt(updatedAt time.Time) {
	note.updatedAt = updatedAt
}

func (note *Note) Autosave(expectedVersion int64, title, markdown string) error {
	if expectedVersion != note.version {
		return ErrVersionConflict
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrNoteTitleRequired
	}
	note.title = title
	note.slug = slugify(title)
	note.markdown = markdown
	note.version++
	return nil
}

func (note *Note) Publish(revisionID string, now time.Time) (Revision, error) {
	revision, err := note.Checkpoint(note.version, revisionID, RevisionReasonPublish, now)
	if err != nil {
		return Revision{}, err
	}
	note.published = &PublishedContent{
		Title:       note.title,
		Slug:        note.slug,
		Markdown:    note.markdown,
		PublishedAt: now,
	}
	return revision, nil
}

func (note *Note) Checkpoint(expectedVersion int64, revisionID string, reason RevisionReason, now time.Time) (Revision, error) {
	if expectedVersion != note.version {
		return Revision{}, ErrVersionConflict
	}
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return Revision{}, ErrRevisionIDRequired
	}
	return Revision{
		ID:        revisionID,
		NoteID:    note.id,
		Title:     note.title,
		Slug:      note.slug,
		Markdown:  note.markdown,
		Reason:    reason,
		CreatedAt: now,
	}, nil
}

func (note *Note) Restore(expectedVersion int64, target Revision, checkpointID string, now time.Time) (Revision, error) {
	checkpoint, err := note.Checkpoint(expectedVersion, checkpointID, RevisionReasonRestore, now)
	if err != nil {
		return Revision{}, err
	}
	if target.NoteID != note.id {
		return Revision{}, ErrRevisionWrongNote
	}
	note.title = target.Title
	note.slug = target.Slug
	note.markdown = target.Markdown
	note.version++
	return checkpoint, nil
}

func (note *Note) Relocate(spaceID, directoryID string) error {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return ErrNoteSpaceIDRequired
	}
	note.spaceID = spaceID
	note.directoryID = strings.TrimSpace(directoryID)
	return nil
}

func (note *Note) MakePrivate() {
	note.published = nil
}

func slugify(title string) string {
	var builder strings.Builder
	separatorPending := false
	for _, character := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if separatorPending && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(character)
			separatorPending = false
			continue
		}
		separatorPending = true
	}
	if builder.Len() == 0 {
		return "note"
	}
	return builder.String()
}
