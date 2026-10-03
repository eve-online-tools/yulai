-- +goose Up
-- Owned by feature/presence. *_at columns are when that part was last fetched.
CREATE TABLE presence (
    character_id    INTEGER PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    online          INTEGER NOT NULL DEFAULT 0,
    last_login      INTEGER,
    last_logout     INTEGER,
    logins          INTEGER,
    solar_system_id INTEGER,
    station_id      INTEGER,
    structure_id    INTEGER,
    ship_type_id    INTEGER,
    ship_item_id    INTEGER,
    ship_name       TEXT,
    online_at       INTEGER,
    location_at     INTEGER,
    ship_at         INTEGER
);

-- +goose Down
DROP TABLE presence;
