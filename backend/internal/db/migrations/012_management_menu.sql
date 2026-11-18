-- "Management" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Management' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('User',   'user',     '/management/user',   10),
    ('Role',   'circle',   '/management/role',   20),
    ('Cabang', 'building', '/management/cabang', 30)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Management' AND p.parent_id IS NULL;
