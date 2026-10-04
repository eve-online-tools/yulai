-- name: GetMeta :one
SELECT * FROM meta;

-- Ship tree: the ship types in it and the static data the tree derives from them. Some modules also carry a
-- ship_tree_group_id, so types are limited to the ship category (6).

-- name: ShipTreeTypes :many
SELECT key, ship_tree_group_id, faction_id, meta_group_id, tech_level
FROM types
WHERE ship_tree_group_id IS NOT NULL AND group_id IN (SELECT key FROM groups WHERE category_id = 6)
ORDER BY key;

-- Required skills 1-5 and their levels (182-184, 277-279, 1285-1289), tech level (422), rig size (1547).
-- name: ShipTreeTypeAttributes :many
SELECT a.parent, a.attribute_id, a.value
FROM type_dogma_dogma_attributes a
JOIN types t ON t.key = a.parent
WHERE t.ship_tree_group_id IS NOT NULL AND t.group_id IN (SELECT key FROM groups WHERE category_id = 6)
  AND a.attribute_id IN (182, 183, 184, 277, 278, 279, 1285, 1286, 1287, 1289, 422, 1547)
ORDER BY a.parent, a.idx;

-- name: ShipTreeMasteries :many
SELECT m.parent AS type_id, m.key AS level, v.value AS certificate_id
FROM masteries_value m
JOIN masteries_value_value v ON v.parent = m.id
JOIN types t ON t.key = m.parent
WHERE t.ship_tree_group_id IS NOT NULL AND t.group_id IN (SELECT key FROM groups WHERE category_id = 6)
ORDER BY m.parent, m.key, v.idx;

-- name: CertificateSkills :many
SELECT * FROM certificates_skill_types ORDER BY parent, key;

-- name: CloneGradeSkills :many
SELECT parent, level, type_id FROM clone_grades_skills ORDER BY parent, idx;

-- name: ShipTreeGroupKeys :many
SELECT key FROM ship_tree_groups ORDER BY key;

-- name: ShipTreeGroupElements :many
SELECT parent, key, value FROM ship_tree_groups_elements ORDER BY parent, key;

-- name: ShipTreeGroupSkills :many
SELECT p.parent AS group_id, p.key AS faction_id, s.key AS skill_id, s.display, s.level
FROM ship_tree_groups_pre_req_skills p
JOIN ship_tree_groups_pre_req_skills_skills s ON s.parent = p.id
ORDER BY p.parent, p.rowid, s.rowid;

-- name: ShipTreeFactions :many
SELECT key, description_en FROM ship_tree_factions ORDER BY key;

-- name: ShipTreeFactionElements :many
SELECT parent, key, value FROM ship_tree_factions_elements ORDER BY parent, key;

-- name: ShipTreeElements :many
SELECT key, name_en FROM ship_tree_elements ORDER BY key;
