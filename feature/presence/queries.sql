-- name: ListOnline :many
SELECT * FROM presence_online;

-- name: GetOnline :one
SELECT * FROM presence_online WHERE character_id = ?;

-- name: GetLocation :one
SELECT * FROM presence_locations WHERE character_id = ?;

-- name: GetShip :one
SELECT * FROM presence_ships WHERE character_id = ?;

-- name: UpsertOnline :one
INSERT INTO presence_online (character_id, online, last_login, last_logout, logins, fetched_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    online = excluded.online,
    last_login = excluded.last_login,
    last_logout = excluded.last_logout,
    logins = excluded.logins,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: UpsertLocation :one
INSERT INTO presence_locations (character_id, solar_system_id, station_id, structure_id, fetched_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    solar_system_id = excluded.solar_system_id,
    station_id = excluded.station_id,
    structure_id = excluded.structure_id,
    fetched_at = excluded.fetched_at
RETURNING *;

-- name: UpsertShip :one
INSERT INTO presence_ships (character_id, ship_item_id, ship_type_id, ship_name, fetched_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    ship_item_id = excluded.ship_item_id,
    ship_type_id = excluded.ship_type_id,
    ship_name = excluded.ship_name,
    fetched_at = excluded.fetched_at
RETURNING *;
