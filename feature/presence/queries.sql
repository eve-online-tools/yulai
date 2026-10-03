-- name: List :many
SELECT * FROM presence ORDER BY character_id;

-- name: Get :one
SELECT * FROM presence WHERE character_id = ?;

-- name: UpsertOnline :exec
INSERT INTO presence (character_id, online, last_login, last_logout, logins, online_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    online = excluded.online,
    last_login = excluded.last_login,
    last_logout = excluded.last_logout,
    logins = excluded.logins,
    online_at = excluded.online_at;

-- name: UpsertLocation :exec
INSERT INTO presence (character_id, solar_system_id, station_id, structure_id, location_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    solar_system_id = excluded.solar_system_id,
    station_id = excluded.station_id,
    structure_id = excluded.structure_id,
    location_at = excluded.location_at;

-- name: UpsertShip :exec
INSERT INTO presence (character_id, ship_type_id, ship_item_id, ship_name, ship_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET
    ship_type_id = excluded.ship_type_id,
    ship_item_id = excluded.ship_item_id,
    ship_name = excluded.ship_name,
    ship_at = excluded.ship_at;
