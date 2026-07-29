CREATE TABLE users
(
    id         bigserial PRIMARY KEY,
    name       text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identities
(
    id             bigserial PRIMARY KEY,
    user_id        bigint      NOT NULL REFERENCES users (id),
    provider       text        NOT NULL,
    provider_id    text        NOT NULL,
    kind           text        NOT NULL,
    email          text        NOT NULL DEFAULT '',
    email_verified boolean     NOT NULL DEFAULT false,
    metadata       jsonb       NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),

    UNIQUE (provider, provider_id)
);

-- The system principal: the user a build triggered by the App itself is attributed to.
-- No login flow speaks the 'system' provider, so no session can ever be minted for it,
-- and identities' UNIQUE (provider, provider_id) keeps the row single.
WITH seeded_user AS (
    INSERT INTO users (name) VALUES ('platform') RETURNING id
)
INSERT INTO identities (user_id, provider, provider_id, kind)
SELECT id, 'system', 'platform', 'system' FROM seeded_user;
