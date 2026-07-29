CREATE TABLE jobs (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind = btrim(kind) AND kind <> ''),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'running', 'succeeded', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts integer NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text,
    last_error text,
    idempotency_key text,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((locked_at IS NULL) = (locked_by IS NULL)),
    CHECK (state = 'running' OR (locked_at IS NULL AND locked_by IS NULL))
);

CREATE UNIQUE INDEX jobs_idempotency_key_key ON jobs (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX jobs_claim_idx ON jobs (available_at, created_at) WHERE state IN ('pending', 'running');
CREATE INDEX jobs_history_idx ON jobs (created_at DESC);
