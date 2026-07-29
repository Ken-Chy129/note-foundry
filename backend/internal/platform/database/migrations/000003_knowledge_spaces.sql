CREATE TABLE knowledge_spaces (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    visibility text NOT NULL CHECK (visibility IN ('private', 'public')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX knowledge_spaces_name_lower_key ON knowledge_spaces (lower(name));
