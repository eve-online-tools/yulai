-- +goose Up
-- Owned by feature/freelance. One table per ESI endpoint, typed like its response.

-- The public /freelance-jobs listing, crawled with cursors. Rows are never removed:
-- finished jobs stay with their final state.
CREATE TABLE freelance_jobs (
    id               JOB_ID    PRIMARY KEY,
    name             TEXT      NOT NULL,
    state            TEXT      NOT NULL,
    progress_current INTEGER   NOT NULL,
    progress_desired INTEGER   NOT NULL,
    reward_initial   REAL,
    reward_remaining REAL,
    last_modified    TIMESTAMP NOT NULL,
    fetched_at       TIMESTAMP NOT NULL
);

-- Cursor state per crawled listing. before_token is set while the backfill of older
-- records is unfinished; after_token is where the next forward crawl starts.
CREATE TABLE freelance_cursors (
    listing      TEXT PRIMARY KEY,
    after_token  TEXT,
    before_token TEXT
);

-- /characters/{id}/freelance-jobs, replaced as a whole on every fetch.
CREATE TABLE character_freelance_jobs (
    character_id     INTEGER   NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    job_id           JOB_ID    NOT NULL,
    name             TEXT      NOT NULL,
    state            TEXT      NOT NULL,
    progress_current INTEGER   NOT NULL,
    progress_desired INTEGER   NOT NULL,
    reward_initial   REAL,
    reward_remaining REAL,
    last_modified    TIMESTAMP NOT NULL,
    fetched_at       TIMESTAMP NOT NULL,
    PRIMARY KEY (character_id, job_id)
);

-- /characters/{id}/freelance-jobs/{job_id}/participation
CREATE TABLE character_freelance_participation (
    character_id  INTEGER   NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    job_id        JOB_ID    NOT NULL,
    contributed   INTEGER   NOT NULL,
    state         TEXT      NOT NULL,
    last_modified TIMESTAMP NOT NULL,
    fetched_at    TIMESTAMP NOT NULL,
    PRIMARY KEY (character_id, job_id)
);

-- +goose Down
DROP TABLE character_freelance_participation;
DROP TABLE character_freelance_jobs;
DROP TABLE freelance_cursors;
DROP TABLE freelance_jobs;
