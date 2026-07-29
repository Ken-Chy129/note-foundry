CREATE TABLE attachments (
    id uuid PRIMARY KEY,
    note_id uuid NOT NULL REFERENCES learning_notes (id) ON DELETE CASCADE,
    storage_key text NOT NULL UNIQUE CHECK (storage_key <> ''),
    original_name text NOT NULL CHECK (original_name <> ''),
    media_type text NOT NULL CHECK (media_type <> ''),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256_hex text NOT NULL CHECK (sha256_hex ~ '^[0-9a-f]{64}$'),
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX attachments_note_created_idx ON attachments (note_id, created_at, id);
CREATE INDEX attachments_public_idx ON attachments (note_id, published_at) WHERE published_at IS NOT NULL;
