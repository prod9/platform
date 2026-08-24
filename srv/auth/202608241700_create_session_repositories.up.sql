CREATE TABLE session_repositories
(
    session_id           bigint      NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    github_repository_id bigint      NOT NULL,
    owner                text        NOT NULL,
    name                 text        NOT NULL,
    permission           text        NOT NULL CHECK (permission IN ('read', 'write')),
    created_at           timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (session_id, github_repository_id),
    UNIQUE (session_id, owner, name)
);
