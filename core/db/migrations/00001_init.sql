-- +goose Up
CREATE TABLE characters (
    id             INTEGER PRIMARY KEY,
    name           TEXT    NOT NULL,
    owner_hash     TEXT    NOT NULL,
    corporation_id INTEGER NOT NULL DEFAULT 0,
    alliance_id    INTEGER,
    account_label  TEXT,
    -- ok | needs_login
    status         TEXT    NOT NULL DEFAULT 'ok',
    status_error   TEXT,
    added_at       INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE TABLE tokens (
    character_id      INTEGER PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    access_token      TEXT    NOT NULL,
    refresh_token_enc BLOB    NOT NULL,
    expires_at        INTEGER NOT NULL,
    -- space separated scp claim of the access token; what the user consented to
    scopes            TEXT    NOT NULL DEFAULT '',
    -- iat claim of the access token: when the token was last (re)issued
    issued_at         INTEGER NOT NULL DEFAULT 0
);

-- User pauses. kind is task or subject. Scheduling itself is not persisted.
CREATE TABLE task_pauses (
    kind  TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (kind, value)
);

-- +goose Down
DROP TABLE task_pauses;
DROP TABLE tokens;
DROP TABLE characters;
