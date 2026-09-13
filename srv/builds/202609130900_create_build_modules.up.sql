ALTER TABLE builds
    ADD COLUMN repo_id bigint NOT NULL REFERENCES repos (id),
    ADD COLUMN manifest_id bigint NOT NULL,
    ADD CONSTRAINT builds_manifest_identity_fk
        FOREIGN KEY (manifest_id, repo_id, sha) REFERENCES repo_manifests (id, repo_id, sha),
    ADD CONSTRAINT builds_id_manifest_unique UNIQUE (id, manifest_id),
    ADD CONSTRAINT builds_retry_predecessor_check
        CHECK ((trigger = 'retry') = (retry_of IS NOT NULL));

ALTER TABLE builds DROP COLUMN owner, DROP COLUMN repo;

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

ALTER TABLE build_events
    ADD COLUMN build_module_id bigint NOT NULL REFERENCES build_modules (id),
    DROP COLUMN build_id,
    DROP COLUMN unit,
    DROP CONSTRAINT build_events_kind_check,
    ADD CONSTRAINT build_events_kind_check CHECK (kind IN (
        'run_start', 'run_done', 'clone_start', 'clone_done', 'config_start',
        'config_done', 'step_start', 'step_done', 'publish_start', 'publish_done'
    ));

CREATE INDEX build_events_module_stream_idx ON build_events (build_module_id, id);
