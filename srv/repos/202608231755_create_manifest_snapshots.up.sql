CREATE TABLE repo_manifests
(
    id           bigserial PRIMARY KEY,
    repo_id      bigint      NOT NULL REFERENCES repos (id),
    sha          text        NOT NULL,
    raw          text        NOT NULL,
    maintainer   text        NOT NULL DEFAULT '',
    repository   text        NOT NULL DEFAULT '',
    platform     text        NOT NULL DEFAULT '',
    local_arch   text        NOT NULL DEFAULT '',
    publish_arch text        NOT NULL DEFAULT '',
    strategy     text        NOT NULL DEFAULT '',
    excludes     text[]      NOT NULL DEFAULT '{}',
    vars         jsonb       NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (repo_id, sha),
    UNIQUE (id, repo_id, sha)
);

CREATE INDEX repo_manifests_current_idx ON repo_manifests (repo_id, created_at DESC);

CREATE TABLE repo_manifest_modules
(
    id           bigserial PRIMARY KEY,
    manifest_id  bigint   NOT NULL REFERENCES repo_manifests (id),
    name         text     NOT NULL,
    workdir      text     NOT NULL DEFAULT '',
    timeout_ns   bigint   NOT NULL DEFAULT 0,
    framework    text     NOT NULL DEFAULT '',
    env          jsonb    NOT NULL DEFAULT '{}',
    port         integer  NOT NULL DEFAULT 0,
    command_name text     NOT NULL DEFAULT '',
    command_args text[]   NOT NULL DEFAULT '{}',
    asset_dirs   text[]   NOT NULL DEFAULT '{}',
    build_dir    text     NOT NULL DEFAULT '',
    go_version   text     NOT NULL DEFAULT '',
    image_name   text     NOT NULL DEFAULT '',
    package_name text     NOT NULL DEFAULT '',

    UNIQUE (manifest_id, name),
    UNIQUE (id, manifest_id)
);
