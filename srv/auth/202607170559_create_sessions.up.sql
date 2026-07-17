CREATE TABLE sessions
(
    id         bigserial PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id),
    token_hash text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
