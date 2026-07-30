CREATE TABLE learning_sources (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('manual', 'url', 'pdf')),
    space_id uuid REFERENCES knowledge_spaces (id) ON DELETE RESTRICT,
    title text NOT NULL CHECK (title = btrim(title) AND title <> ''),
    capture_note text NOT NULL DEFAULT '',
    current_content text NOT NULL DEFAULT '',
    processing_status text NOT NULL CHECK (processing_status IN ('pending', 'processing', 'ready', 'failed')),
    failure_message text,
    trashed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind <> 'manual' OR processing_status = 'ready')
);

CREATE INDEX learning_sources_inbox_updated_idx
    ON learning_sources (updated_at DESC, id DESC)
    WHERE space_id IS NULL AND trashed_at IS NULL;

CREATE INDEX learning_sources_space_updated_idx
    ON learning_sources (space_id, updated_at DESC, id DESC)
    WHERE space_id IS NOT NULL AND trashed_at IS NULL;
