-- "Screening Covid 19" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Screening Covid 19' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, 'circle', c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Antigen', '/screening-covid-19/antigen', 10),
    ('PCR',     '/screening-covid-19/pcr',     20)
) AS c (title, path, sort_order)
WHERE p.title = 'Screening Covid 19' AND p.parent_id IS NULL;
