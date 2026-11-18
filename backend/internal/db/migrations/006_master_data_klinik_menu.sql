-- "Master Data Klinik" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Master Data Klinik' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Obat',              'circle', '/master-data-klinik/obat',              10),
    ('Tindakan',          'circle', '/master-data-klinik/tindakan',          20),
    ('Penunjang',         'circle', '/master-data-klinik/penunjang',         30),
    ('Pasien',            'circle', '/master-data-klinik/pasien',            40),
    ('Dokter',            'user',   '/master-data-klinik/dokter',            50),
    ('Jaminan Kesehatan', 'circle', '/master-data-klinik/jaminan-kesehatan', 60),
    ('Distributor',       'circle', '/master-data-klinik/distributor',       70),
    ('Principal',         'circle', '/master-data-klinik/principal',         80),
    ('ICD',               'circle', '/master-data-klinik/icd',               90),
    ('Penjual OTC',       'user',   '/master-data-klinik/penjual-otc',      100),
    ('Satu Sehat',        'circle', '/master-data-klinik/satu-sehat',       110)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Master Data Klinik' AND p.parent_id IS NULL;
