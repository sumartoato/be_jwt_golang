-- "Report Satu Sehat" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Report Satu Sehat' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, 'EncounterKunjungan', 'circle', '/report-satu-sehat/encounter-kunjungan', 10
FROM menus p
WHERE p.title = 'Report Satu Sehat' AND p.parent_id IS NULL;
