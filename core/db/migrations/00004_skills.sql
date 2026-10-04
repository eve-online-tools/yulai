-- +goose Up
-- Owned by feature/skills. One table per ESI endpoint, typed like its response.
CREATE TABLE skill_attributes (
    character_id                INTEGER   PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    charisma                    INTEGER   NOT NULL,
    intelligence                INTEGER   NOT NULL,
    memory                      INTEGER   NOT NULL,
    perception                  INTEGER   NOT NULL,
    willpower                   INTEGER   NOT NULL,
    bonus_remaps                INTEGER,
    last_remap_date             TIMESTAMP,
    accrued_remap_cooldown_date TIMESTAMP,
    fetched_at                  TIMESTAMP NOT NULL
);

-- The skills response: totals here, one row per skill in skills.
CREATE TABLE skill_totals (
    character_id   INTEGER   PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    total_sp       INTEGER   NOT NULL,
    unallocated_sp INTEGER,
    fetched_at     TIMESTAMP NOT NULL
);

CREATE TABLE skills (
    character_id         INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id             INTEGER NOT NULL,
    active_skill_level   INTEGER NOT NULL,
    trained_skill_level  INTEGER NOT NULL,
    skillpoints_in_skill INTEGER NOT NULL,
    PRIMARY KEY (character_id, skill_id)
);

-- Replaced as a whole on every fetch. An empty queue has no rows.
CREATE TABLE skill_queue (
    character_id      INTEGER   NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    queue_position    INTEGER   NOT NULL,
    skill_id          TYPE_ID   NOT NULL,
    finished_level    INTEGER   NOT NULL,
    start_date        TIMESTAMP,
    finish_date       TIMESTAMP,
    training_start_sp INTEGER,
    level_start_sp    INTEGER,
    level_end_sp      INTEGER,
    fetched_at        TIMESTAMP NOT NULL,
    PRIMARY KEY (character_id, queue_position)
);

-- +goose Down
DROP TABLE skill_queue;
DROP TABLE skills;
DROP TABLE skill_totals;
DROP TABLE skill_attributes;
