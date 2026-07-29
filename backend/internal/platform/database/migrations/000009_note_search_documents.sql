CREATE TABLE note_search_documents (
    note_id uuid PRIMARY KEY REFERENCES learning_notes (id) ON DELETE CASCADE,
    current_title text NOT NULL,
    current_headings text NOT NULL DEFAULT '',
    current_body text NOT NULL,
    current_tags text NOT NULL DEFAULT '',
    published_available boolean NOT NULL DEFAULT false,
    published_title text NOT NULL DEFAULT '',
    published_headings text NOT NULL DEFAULT '',
    published_body text NOT NULL DEFAULT '',
    published_tags text NOT NULL DEFAULT '',
    current_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple'::regconfig, current_title), 'A') ||
        setweight(to_tsvector('simple'::regconfig, current_headings), 'B') ||
        setweight(to_tsvector('simple'::regconfig, current_tags), 'C') ||
        setweight(to_tsvector('simple'::regconfig, current_body), 'D')
    ) STORED,
    published_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple'::regconfig, published_title), 'A') ||
        setweight(to_tsvector('simple'::regconfig, published_headings), 'B') ||
        setweight(to_tsvector('simple'::regconfig, published_tags), 'C') ||
        setweight(to_tsvector('simple'::regconfig, published_body), 'D')
    ) STORED,
    current_text text GENERATED ALWAYS AS (
        current_title || E'\n' || current_headings || E'\n' || current_tags || E'\n' || current_body
    ) STORED,
    published_text text GENERATED ALWAYS AS (
        published_title || E'\n' || published_headings || E'\n' || published_tags || E'\n' || published_body
    ) STORED,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX note_search_documents_current_vector_idx ON note_search_documents USING gin (current_vector);
CREATE INDEX note_search_documents_published_vector_idx ON note_search_documents USING gin (published_vector) WHERE published_available;
CREATE INDEX note_search_documents_current_trgm_idx ON note_search_documents USING gin (current_text gin_trgm_ops);
CREATE INDEX note_search_documents_published_trgm_idx ON note_search_documents USING gin (published_text gin_trgm_ops) WHERE published_available;
