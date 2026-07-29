CREATE TABLE note_tags (
    note_id uuid NOT NULL REFERENCES learning_notes (id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (note_id, tag_id)
);

CREATE INDEX note_tags_tag_note_idx ON note_tags (tag_id, note_id);
