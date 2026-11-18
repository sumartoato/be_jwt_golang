-- "Medical Check Up" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Medical Check Up' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, 'Pemeriksaan', 'circle', '/medical-check-up/pemeriksaan', 10
FROM menus p
WHERE p.title = 'Medical Check Up' AND p.parent_id IS NULL;
