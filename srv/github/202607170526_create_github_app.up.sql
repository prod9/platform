CREATE TABLE github_app
(
    id             int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    app_id         bigint      NOT NULL,
    slug           text        NOT NULL,
    private_key    text        NOT NULL,
    webhook_secret text        NOT NULL,
    client_id      text        NOT NULL,
    client_secret  text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
