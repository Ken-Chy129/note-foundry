package notes

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

var (
	ErrNoteIDRequired      = errors.New("Learning Note id is required")
	ErrNoteSpaceIDRequired = errors.New("Learning Note Knowledge Space id is required")
	ErrNoteTitleRequired   = errors.New("Learning Note title is required")
	ErrRevisionIDRequired  = errors.New("Note Revision id is required")
	ErrVersionConflict     = errors.New("Learning Note was changed by another save")
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
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return Revision{}, ErrRevisionIDRequired
	}
	revision := Revision{
		ID:        revisionID,
		NoteID:    note.id,
		Title:     note.title,
		Slug:      note.slug,
		Markdown:  note.markdown,
		Reason:    RevisionReasonPublish,
		CreatedAt: now,
	}
	note.published = &PublishedContent{
		Title:       note.title,
		Slug:        note.slug,
		Markdown:    note.markdown,
		PublishedAt: now,
	}
	return revision, nil
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
