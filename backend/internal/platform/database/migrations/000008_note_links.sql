CREATE TABLE note_links (
    source_note_id uuid NOT NULL REFERENCES learning_notes (id) ON DELETE CASCADE,
    target_note_id uuid NOT NULL REFERENCES learning_notes (id) ON DELETE CASCADE,
    content_state text NOT NULL CHECK (content_state IN ('current', 'published')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source_note_id, target_note_id, content_state)
);

CREATE INDEX note_links_target_state_source_idx ON note_links (target_note_id, content_state, source_note_id);
