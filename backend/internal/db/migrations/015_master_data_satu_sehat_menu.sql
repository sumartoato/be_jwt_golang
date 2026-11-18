-- "Master Data Satu Sehat" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Master Data Satu Sehat' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, 'Organisasi & Lokasi', 'circle', '/master-data-satu-sehat/organisasi-lokasi', 10
FROM menus p
WHERE p.title = 'Master Data Satu Sehat' AND p.parent_id IS NULL;
