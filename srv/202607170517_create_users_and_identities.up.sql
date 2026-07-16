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
