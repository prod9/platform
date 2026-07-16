CREATE TABLE builds
(
    id         bigserial PRIMARY KEY,
    owner      text        NOT NULL,
    repo       text        NOT NULL,
    clone_url  text        NOT NULL,
    tag        text        NOT NULL,
    sha        text        NOT NULL,
    status     text        NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    error      text        NOT NULL DEFAULT '',
    image      text        NOT NULL DEFAULT '',
    digest     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
