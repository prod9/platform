ALTER TABLE repo_manifests
    RENAME CONSTRAINT repo_manifests_publish_policy_check
    TO repo_manifests_server_publish_check;

ALTER TABLE repo_manifests
    RENAME COLUMN publish_policy TO server_publish;
