ALTER TABLE learning_sources
    ADD COLUMN original_url text,
    ADD COLUMN normalized_url text;

ALTER TABLE learning_sources
    ADD CONSTRAINT learning_sources_url_metadata_check CHECK (
        (kind = 'url' AND original_url IS NOT NULL AND normalized_url IS NOT NULL)
        OR (kind <> 'url' AND original_url IS NULL AND normalized_url IS NULL)
    );

CREATE UNIQUE INDEX learning_sources_normalized_url_key
    ON learning_sources (normalized_url)
    WHERE kind = 'url' AND trashed_at IS NULL;
