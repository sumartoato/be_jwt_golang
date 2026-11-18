-- "Front Office Klinik" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Front Office Klinik' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Pendaftaran Pasien', 'user-circle', '/front-office-klinik/pendaftaran-pasien', 10),
    ('Riwayat Pasien',     'user',        '/front-office-klinik/riwayat-pasien',     20)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Front Office Klinik' AND p.parent_id IS NULL;
