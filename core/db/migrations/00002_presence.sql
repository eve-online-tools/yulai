-- +goose Up
-- Owned by feature/presence. One table per ESI endpoint, typed like its response.
CREATE TABLE presence_online (
    character_id INTEGER   PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    online       BOOLEAN   NOT NULL,
    last_login   TIMESTAMP,
    last_logout  TIMESTAMP,
    logins       INTEGER,
    fetched_at   TIMESTAMP NOT NULL
);

CREATE TABLE presence_locations (
    character_id    INTEGER         PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    solar_system_id SOLAR_SYSTEM_ID NOT NULL,
    station_id      STATION_ID,
    structure_id    ITEM_ID,
    fetched_at      TIMESTAMP       NOT NULL
);

CREATE TABLE presence_ships (
    character_id INTEGER   PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    ship_item_id ITEM_ID   NOT NULL,
    ship_type_id TYPE_ID   NOT NULL,
    ship_name    TEXT      NOT NULL,
    fetched_at   TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE presence_ships;
DROP TABLE presence_locations;
DROP TABLE presence_online;
