-- "Stok Farmasi" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Stok Farmasi' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, 'circle', c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Stok Opname',    '/stok-farmasi/stok-opname',    10),
    ('Stok Adjusment', '/stok-farmasi/stok-adjusment', 20),
    ('Stok Alert',     '/stok-farmasi/stok-alert',     30),
    ('Stok Trace',     '/stok-farmasi/stok-trace',     40)
) AS c (title, path, sort_order)
WHERE p.title = 'Stok Farmasi' AND p.parent_id IS NULL;
