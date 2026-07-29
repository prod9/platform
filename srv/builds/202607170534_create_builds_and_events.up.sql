CREATE TABLE builds
(
    id         bigserial PRIMARY KEY,
    trigger    text        NOT NULL
        CHECK (trigger IN ('github-push', 'webui', 'cli', 'retry')),
    retry_of   bigint REFERENCES builds (id),
    user_id    bigint      NOT NULL REFERENCES users (id),
    owner      text        NOT NULL,
    repo       text        NOT NULL,
    clone_url  text        NOT NULL,
    ref        text        NOT NULL,
    sha        text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Append-only transcription of the engine's Observer callbacks; a build's whole state is
-- a fold of these rows, which is why builds carries no status of its own. `at` is the
-- engine's own callback time, so elapsed time survives a slow writer.
CREATE TABLE build_events
(
    id         bigserial PRIMARY KEY,
    build_id   bigint      NOT NULL REFERENCES builds (id),
    kind       text        NOT NULL
        CHECK (kind IN ('step_started', 'step_done', 'image_built', 'published', 'run_done')),
    unit       text        NOT NULL DEFAULT '',
    step       text        NOT NULL DEFAULT '',
    at         timestamptz NOT NULL,
    error      text        NOT NULL DEFAULT '',
    image      text        NOT NULL DEFAULT '',
    hash       text        NOT NULL DEFAULT '',
    stdout     text        NOT NULL DEFAULT '',
    stderr     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX build_events_stream_idx ON build_events (build_id, id);
