-- name: List :many
SELECT c.id, c.name, c.corporation_id, c.alliance_id, c.status, c.status_error,
    COALESCE(t.scopes, '') AS scopes, t.expires_at AS token_expires_at, t.issued_at AS token_issued_at
FROM characters c
LEFT JOIN tokens t ON t.character_id = c.id
ORDER BY c.name;

-- name: Upsert :exec
INSERT INTO characters (id, name, owner_hash, corporation_id, alliance_id, status, status_error, added_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'ok', NULL, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    owner_hash = excluded.owner_hash,
    corporation_id = excluded.corporation_id,
    alliance_id = excluded.alliance_id,
    status = 'ok',
    status_error = NULL,
    updated_at = excluded.updated_at;

-- name: SetStatus :exec
UPDATE characters SET status = ?, status_error = ?, updated_at = ? WHERE id = ?;

-- name: Delete :exec
DELETE FROM characters WHERE id = ?;
