package attachments

import "time"

type Attachment struct {
	ID           string
	NoteID       string
	StorageKey   string
	OriginalName string
	MediaType    string
	SizeBytes    int64
	SHA256Hex    string
	PublishedAt  *time.Time
	CreatedAt    time.Time
}
