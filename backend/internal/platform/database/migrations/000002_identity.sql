CREATE TABLE oauth_states (
    token_digest bytea PRIMARY KEY CHECK (octet_length(token_digest) = 32),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX oauth_states_expires_at_idx ON oauth_states (expires_at);

CREATE TABLE owner_sessions (
    token_digest bytea PRIMARY KEY CHECK (octet_length(token_digest) = 32),
    github_user_id bigint NOT NULL CHECK (github_user_id > 0),
    github_login text NOT NULL CHECK (github_login <> ''),
    github_avatar_url text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX owner_sessions_expires_at_idx ON owner_sessions (expires_at);
