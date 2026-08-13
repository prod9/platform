-- Registration only: this repo is onboarded to build here. It is a product fact, not
-- a permission: no role, no access bit, no cached GitHub state lives on it
-- (docs/spec/platform-server.md §Repos are registered, visibility is live).
CREATE TABLE repos (
  id            BIGSERIAL PRIMARY KEY,
  owner         TEXT NOT NULL,
  repo          TEXT NOT NULL,
  registered_by BIGINT NOT NULL REFERENCES users (id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

  UNIQUE (owner, repo)
);
