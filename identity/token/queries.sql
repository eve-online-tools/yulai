-- name: UpsertToken :exec
INSERT INTO tokens (character_id, access_token, refresh_token_enc, expires_at, scopes, issued_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    access_token = excluded.access_token,
    refresh_token_enc = excluded.refresh_token_enc,
    expires_at = excluded.expires_at,
    scopes = excluded.scopes,
    issued_at = excluded.issued_at;

-- name: GetToken :one
SELECT * FROM tokens WHERE character_id = ?;

-- name: GetScopes :one
SELECT scopes FROM tokens WHERE character_id = ?;
