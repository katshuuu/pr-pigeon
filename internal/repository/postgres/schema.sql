CREATE TABLE IF NOT EXISTS subscriptions (
    chat_id      BIGINT      PRIMARY KEY,
    github_login TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GitHub logins are case-insensitive, so we look them up by lower().
CREATE INDEX IF NOT EXISTS subscriptions_login_idx ON subscriptions (lower(github_login));
