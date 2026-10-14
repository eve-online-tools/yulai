-- name: GetJob :one
SELECT * FROM freelance_jobs WHERE id = ?;

-- name: UpsertJob :one
INSERT INTO freelance_jobs (
    id, name, state, progress_current, progress_desired,
    reward_initial, reward_remaining, last_modified, fetched_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    state = excluded.state,
    progress_current = excluded.progress_current,
    progress_desired = excluded.progress_desired,
    reward_initial = excluded.reward_initial,
    reward_remaining = excluded.reward_remaining,
    last_modified = excluded.last_modified,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: ListJobs :many
SELECT * FROM freelance_jobs ORDER BY last_modified DESC;

-- name: GetCursor :one
SELECT * FROM freelance_cursors WHERE listing = ?;

-- name: UpsertCursor :exec
INSERT INTO freelance_cursors (listing, after_token, before_token)
VALUES (?, ?, ?)
ON CONFLICT(listing) DO UPDATE SET
    after_token = excluded.after_token,
    before_token = excluded.before_token;

-- name: ListCharacterJobs :many
SELECT * FROM character_freelance_jobs WHERE character_id = ? ORDER BY last_modified DESC, job_id;

-- name: GetCharacterJob :one
SELECT * FROM character_freelance_jobs WHERE character_id = ? AND job_id = ?;

-- name: DeleteCharacterJobs :exec
DELETE FROM character_freelance_jobs WHERE character_id = ?;

-- name: InsertCharacterJob :one
INSERT INTO character_freelance_jobs (
    character_id, job_id, name, state, progress_current, progress_desired,
    reward_initial, reward_remaining, last_modified, fetched_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetParticipation :one
SELECT * FROM character_freelance_participation WHERE character_id = ? AND job_id = ?;

-- name: ListParticipation :many
SELECT * FROM character_freelance_participation WHERE character_id = ? ORDER BY job_id;

-- name: UpsertParticipation :one
INSERT INTO character_freelance_participation (
    character_id, job_id, contributed, state, last_modified, fetched_at
)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id, job_id) DO UPDATE SET
    contributed = excluded.contributed,
    state = excluded.state,
    last_modified = excluded.last_modified,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: DeleteParticipation :execrows
DELETE FROM character_freelance_participation WHERE character_id = ? AND job_id = ?;

-- Participation of jobs no longer on the character's listing.
-- name: DeleteStaleParticipation :exec
DELETE FROM character_freelance_participation
WHERE character_freelance_participation.character_id = ?
  AND job_id NOT IN (SELECT j.job_id FROM character_freelance_jobs j WHERE j.character_id = character_freelance_participation.character_id);
