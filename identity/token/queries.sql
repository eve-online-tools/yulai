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

-- name: RotateToken :execrows
-- Compare-and-swap on the sealed refresh token, so a refresh started before a new
-- login cannot overwrite it.
UPDATE tokens SET access_token = ?, refresh_token_enc = ?, expires_at = ?
WHERE character_id = sqlc.arg(character_id) AND refresh_token_enc = sqlc.arg(prev_refresh_token_enc);

-- name: SetScopes :exec
UPDATE tokens SET scopes = ?, issued_at = ?
WHERE character_id = sqlc.arg(character_id) AND refresh_token_enc = sqlc.arg(refresh_token_enc);
