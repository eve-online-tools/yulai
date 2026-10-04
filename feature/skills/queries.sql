-- name: GetAttributes :one
SELECT * FROM character_attributes WHERE character_id = ?;

-- name: UpsertAttributes :one
INSERT INTO character_attributes (
    character_id, charisma, intelligence, memory, perception, willpower,
    bonus_remaps, last_remap_date, accrued_remap_cooldown_date, fetched_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    charisma = excluded.charisma,
    intelligence = excluded.intelligence,
    memory = excluded.memory,
    perception = excluded.perception,
    willpower = excluded.willpower,
    bonus_remaps = excluded.bonus_remaps,
    last_remap_date = excluded.last_remap_date,
    accrued_remap_cooldown_date = excluded.accrued_remap_cooldown_date,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: GetTotals :one
SELECT * FROM skill_totals WHERE character_id = ?;

-- name: UpsertTotals :one
INSERT INTO skill_totals (character_id, total_sp, unallocated_sp, fetched_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    total_sp = excluded.total_sp,
    unallocated_sp = excluded.unallocated_sp,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: ListSkills :many
SELECT * FROM skills WHERE character_id = ? ORDER BY skill_id;

-- name: DeleteSkills :exec
DELETE FROM skills WHERE character_id = ?;

-- name: InsertSkill :one
INSERT INTO skills (character_id, skill_id, active_skill_level, trained_skill_level, skillpoints_in_skill)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListQueue :many
SELECT * FROM skill_queue WHERE character_id = ? ORDER BY queue_position;

-- name: DeleteQueue :exec
DELETE FROM skill_queue WHERE character_id = ?;

-- name: InsertQueueEntry :one
INSERT INTO skill_queue (
    character_id, queue_position, skill_id, finished_level, start_date, finish_date,
    training_start_sp, level_start_sp, level_end_sp, fetched_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;
