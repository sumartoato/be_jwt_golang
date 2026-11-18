-- Top-level "Report" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Report' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Invoice',        'dollar', '/report/invoice',        10),
    ('Fee Rujukan',    'dollar', '/report/fee-rujukan',    20),
    ('Report Mcu Raw', 'file',   '/report/report-mcu-raw', 30)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Report' AND p.parent_id IS NULL;
