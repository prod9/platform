CREATE TABLE installations (
  id                   integer     PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  org_id               bigint      NOT NULL,
  org_login            text        NOT NULL DEFAULT '',
  installation_id      bigint      NOT NULL,
  installed_by_user_id bigint      NOT NULL,
  installed_by_login   text        NOT NULL DEFAULT '',
  installed_at         timestamptz NOT NULL DEFAULT now()
);
