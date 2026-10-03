-- name: GetCharacter :one
SELECT * FROM character_sheets WHERE character_id = ?;

-- name: GetCorporation :one
SELECT * FROM corporations WHERE id = ?;

-- name: GetAlliance :one
SELECT * FROM alliances WHERE id = ?;

-- name: UpsertCharacter :one
INSERT INTO character_sheets (character_id, corporation_id, alliance_id, faction_id, birthday, bloodline_id,
    race_id, gender, security_status, title, description, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    corporation_id = excluded.corporation_id,
    alliance_id = excluded.alliance_id,
    faction_id = excluded.faction_id,
    birthday = excluded.birthday,
    bloodline_id = excluded.bloodline_id,
    race_id = excluded.race_id,
    gender = excluded.gender,
    security_status = excluded.security_status,
    title = excluded.title,
    description = excluded.description,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: UpsertCorporation :one
INSERT INTO corporations (id, name, ticker, alliance_id, ceo_id, creator_id, enlisted_faction_id, home_station_id,
    date_founded, member_count, tax_rate, war_eligible, url, description, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    ticker = excluded.ticker,
    alliance_id = excluded.alliance_id,
    ceo_id = excluded.ceo_id,
    creator_id = excluded.creator_id,
    enlisted_faction_id = excluded.enlisted_faction_id,
    home_station_id = excluded.home_station_id,
    date_founded = excluded.date_founded,
    member_count = excluded.member_count,
    tax_rate = excluded.tax_rate,
    war_eligible = excluded.war_eligible,
    url = excluded.url,
    description = excluded.description,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: UpsertAlliance :one
INSERT INTO alliances (id, name, ticker, creator_id, creator_corporation_id, executor_corporation_id,
    faction_id, date_founded, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    ticker = excluded.ticker,
    creator_id = excluded.creator_id,
    creator_corporation_id = excluded.creator_corporation_id,
    executor_corporation_id = excluded.executor_corporation_id,
    faction_id = excluded.faction_id,
    date_founded = excluded.date_founded,
    fetched_at = excluded.fetched_at
RETURNING *;
