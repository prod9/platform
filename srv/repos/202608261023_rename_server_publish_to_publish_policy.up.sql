ALTER TABLE repo_manifests
    RENAME COLUMN server_publish TO publish_policy;

ALTER TABLE repo_manifests
    RENAME CONSTRAINT repo_manifests_server_publish_check
    TO repo_manifests_publish_policy_check;
