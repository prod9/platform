-- Historical aggregates have no recorded manifest/module intent. Preserve them intact
-- rather than assigning fictional identities (platform-server.md, Build tables).
LOCK TABLE builds, build_events IN ACCESS EXCLUSIVE MODE;
CREATE SCHEMA build_history;
ALTER TABLE builds SET SCHEMA build_history;
ALTER TABLE build_events SET SCHEMA build_history;

CREATE TABLE builds
(
    id          bigserial PRIMARY KEY,
    trigger     text NOT NULL CHECK (trigger IN ('github-push', 'webui', 'cli', 'retry')),
    retry_of    bigint REFERENCES builds (id),
    user_id     bigint NOT NULL REFERENCES users (id),
    repo_id     bigint NOT NULL REFERENCES repos (id),
    manifest_id bigint NOT NULL,
    clone_url   text NOT NULL,
    ref         text NOT NULL,
    sha         text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT builds_manifest_identity_fk
        FOREIGN KEY (manifest_id, repo_id, sha) REFERENCES repo_manifests (id, repo_id, sha),
    CONSTRAINT builds_id_manifest_unique UNIQUE (id, manifest_id),
    CONSTRAINT builds_retry_predecessor_check
        CHECK ((trigger = 'retry') = (retry_of IS NOT NULL))
);

-- Preserve allocated IDs as well as rows: an old build URL must never name a new build.
SELECT setval('builds_id_seq',
    GREATEST(last_value, COALESCE((SELECT max(id) FROM build_history.builds), 1)),
    is_called OR EXISTS (SELECT 1 FROM build_history.builds))
FROM build_history.builds_id_seq;

CREATE TABLE build_modules
(
    id                 bigserial PRIMARY KEY,
    build_id           bigint      NOT NULL,
    manifest_id        bigint      NOT NULL,
    manifest_module_id bigint      NOT NULL,
    claimed_at         timestamptz,
    claimed_by         text        NOT NULL DEFAULT '',
    engine_host        text        NOT NULL DEFAULT '',
    engine_assigned_at timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),

    UNIQUE (build_id, manifest_module_id),
    FOREIGN KEY (build_id, manifest_id) REFERENCES builds (id, manifest_id),
    FOREIGN KEY (manifest_module_id, manifest_id) REFERENCES repo_manifest_modules (id, manifest_id),
    CHECK ((claimed_at IS NULL) = (claimed_by = '')),
    CHECK ((engine_assigned_at IS NULL) = (engine_host = '')),
    CHECK (engine_assigned_at IS NULL OR claimed_at IS NOT NULL)
);

CREATE INDEX build_modules_unclaimed_idx ON build_modules (id) WHERE claimed_at IS NULL;

CREATE TABLE build_events
(
    id              bigserial PRIMARY KEY,
    build_module_id bigint NOT NULL REFERENCES build_modules (id),
    kind            text NOT NULL CHECK (kind IN (
        'run_start', 'run_done', 'clone_start', 'clone_done', 'config_start',
        'config_done', 'step_start', 'step_done', 'publish_start', 'publish_done'
    )),
    step            text NOT NULL DEFAULT '',
    at              timestamptz NOT NULL,
    error           text NOT NULL DEFAULT '',
    image           text NOT NULL DEFAULT '',
    hash            text NOT NULL DEFAULT '',
    stdout          text NOT NULL DEFAULT '',
    stderr          text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX build_events_module_stream_idx ON build_events (build_module_id, id);

SELECT setval('build_events_id_seq',
    GREATEST(last_value, COALESCE((SELECT max(id) FROM build_history.build_events), 1)),
    is_called OR EXISTS (SELECT 1 FROM build_history.build_events))
FROM build_history.build_events_id_seq;
