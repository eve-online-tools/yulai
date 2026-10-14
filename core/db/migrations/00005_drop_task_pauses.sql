-- +goose Up
-- The scheduler no longer has user pauses.
DROP TABLE task_pauses;

-- +goose Down
CREATE TABLE task_pauses (
    kind  TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (kind, value)
);
