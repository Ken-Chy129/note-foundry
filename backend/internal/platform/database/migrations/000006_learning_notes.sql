CREATE TABLE learning_notes (
    id uuid PRIMARY KEY,
    space_id uuid NOT NULL REFERENCES knowledge_spaces (id) ON DELETE RESTRICT,
    directory_id uuid,
    title text NOT NULL CHECK (title = btrim(title) AND title <> ''),
    slug text NOT NULL CHECK (slug <> ''),
    current_markdown text NOT NULL,
    current_version bigint NOT NULL DEFAULT 1 CHECK (current_version >= 1),
    published_title text,
    published_slug text,
    published_markdown text,
    published_at timestamptz,
    trashed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (directory_id, space_id) REFERENCES directories (id, space_id) ON DELETE RESTRICT,
    CHECK (
        (published_title IS NULL AND published_slug IS NULL AND published_markdown IS NULL AND published_at IS NULL)
        OR
        (published_title IS NOT NULL AND published_slug IS NOT NULL AND published_markdown IS NOT NULL AND published_at IS NOT NULL)
    )
);

CREATE INDEX learning_notes_space_directory_idx ON learning_notes (space_id, directory_id) WHERE trashed_at IS NULL;
CREATE INDEX learning_notes_trashed_at_idx ON learning_notes (trashed_at) WHERE trashed_at IS NOT NULL;

CREATE TABLE note_revisions (
    id uuid PRIMARY KEY,
    note_id uuid NOT NULL REFERENCES learning_notes (id) ON DELETE CASCADE,
    title text NOT NULL,
    slug text NOT NULL,
    markdown text NOT NULL,
    reason text NOT NULL CHECK (reason IN ('publish', 'restore', 'manual')),
    created_at timestamptz NOT NULL
);

CREATE INDEX note_revisions_note_created_idx ON note_revisions (note_id, created_at DESC, id);
