CREATE TABLE directories (
    id uuid PRIMARY KEY,
    space_id uuid NOT NULL REFERENCES knowledge_spaces (id) ON DELETE RESTRICT,
    parent_id uuid,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> id),
    UNIQUE (id, space_id),
    FOREIGN KEY (parent_id, space_id) REFERENCES directories (id, space_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX directories_sibling_name_lower_key
    ON directories (space_id, parent_id, lower(name)) NULLS NOT DISTINCT;

CREATE INDEX directories_space_parent_idx ON directories (space_id, parent_id);
