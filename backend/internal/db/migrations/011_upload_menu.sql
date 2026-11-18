-- "Upload" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Upload' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, 'copy', c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Files',            '/upload/files',            10),
    ('Multi Invoice',    '/upload/multi-invoice',    20),
    ('Check Status Mcu', '/upload/check-status-mcu', 30),
    ('Menu',             '/upload/menu',             40),
    ('MenuLog',          '/upload/menu-log',         50)
) AS c (title, path, sort_order)
WHERE p.title = 'Upload' AND p.parent_id IS NULL;
