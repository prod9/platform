ALTER TABLE builds
    ADD COLUMN owner text NOT NULL,
    ADD COLUMN repo text NOT NULL;

ALTER TABLE build_events
    ADD COLUMN build_id bigint NOT NULL REFERENCES builds (id),
    ADD COLUMN unit text NOT NULL DEFAULT '',
    DROP COLUMN build_module_id,
    DROP CONSTRAINT build_events_kind_check,
    ADD CONSTRAINT build_events_kind_check CHECK (kind IN (
        'step_started', 'step_done', 'image_built', 'published', 'run_done'
    ));

DROP TABLE build_modules;

ALTER TABLE builds
    DROP CONSTRAINT builds_manifest_identity_fk,
    DROP CONSTRAINT builds_id_manifest_unique,
    DROP CONSTRAINT builds_retry_predecessor_check,
    DROP COLUMN repo_id,
    DROP COLUMN manifest_id;

CREATE INDEX build_events_stream_idx ON build_events (build_id, id);
