ALTER TABLE repo_manifests
    ADD COLUMN server_publish text;

UPDATE repo_manifests AS manifest
SET server_publish = CASE
    WHEN repo.repo ~ '(^|-)infra$' THEN 'always'
    ELSE 'tags'
END
FROM repos AS repo
WHERE repo.id = manifest.repo_id;

ALTER TABLE repo_manifests
    ALTER COLUMN server_publish SET NOT NULL,
    ADD CONSTRAINT repo_manifests_server_publish_check
        CHECK (server_publish IN ('always', 'tags', 'never'));
