-- +goose Up
-- Owned by feature/charactersheet. One table per ESI endpoint, typed like its response.
CREATE TABLE character_sheets (
    character_id    INTEGER        PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    corporation_id  CORPORATION_ID NOT NULL,
    alliance_id     ALLIANCE_ID,
    faction_id      FACTION_ID,
    birthday        TIMESTAMP      NOT NULL,
    bloodline_id    BLOODLINE_ID   NOT NULL,
    race_id         RACE_ID        NOT NULL,
    gender          TEXT           NOT NULL,
    security_status REAL,
    title           TEXT,
    description     TEXT,
    fetched_at      TIMESTAMP      NOT NULL
);

-- Corporations and alliances our characters are or were in. Rows are kept when they leave.
CREATE TABLE corporations (
    id                  CORPORATION_ID PRIMARY KEY,
    name                TEXT           NOT NULL,
    ticker              TEXT           NOT NULL,
    alliance_id         ALLIANCE_ID,
    ceo_id              CHARACTER_ID,
    creator_id          CHARACTER_ID,
    enlisted_faction_id FACTION_ID,
    home_station_id     STATION_ID     NOT NULL,
    date_founded        TIMESTAMP,
    member_count        INTEGER        NOT NULL,
    tax_rate            REAL           NOT NULL,
    war_eligible        BOOLEAN        NOT NULL,
    url                 TEXT,
    description         TEXT           NOT NULL,
    fetched_at          TIMESTAMP      NOT NULL
);

CREATE TABLE alliances (
    id                      ALLIANCE_ID    PRIMARY KEY,
    name                    TEXT           NOT NULL,
    ticker                  TEXT           NOT NULL,
    creator_id              CHARACTER_ID   NOT NULL,
    creator_corporation_id  CORPORATION_ID NOT NULL,
    executor_corporation_id CORPORATION_ID,
    faction_id              FACTION_ID,
    date_founded            TIMESTAMP      NOT NULL,
    fetched_at              TIMESTAMP      NOT NULL
);

-- +goose Down
DROP TABLE alliances;
DROP TABLE corporations;
DROP TABLE character_sheets;
