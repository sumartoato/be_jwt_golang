-- Sidebar menu, editable in the database.
--   parent_id  NULL = top level; otherwise the id of the group it belongs to
--   icon       icon name understood by the frontend (see frontend/src/layout/icons.tsx);
--              unknown names fall back to a dot
--   path       frontend route; NULL for a group that only opens its children
--   sort_order lower comes first among siblings
--   is_active  false hides the item (and everything under it)
-- Changes show up after the user reloads the page; no restart needed.
CREATE TABLE IF NOT EXISTS menus (
    id         SERIAL PRIMARY KEY,
    parent_id  INT REFERENCES menus (id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    icon       TEXT NOT NULL DEFAULT '',
    path       TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS menus_parent_id_idx ON menus (parent_id);

-- Initial menu. Runs once (tracked in schema_migrations).
INSERT INTO menus (title, icon, path, sort_order) VALUES
    ('Home',                   'home',      '/dashboard',               10),
    ('Master Data',            'database',  NULL,                       20),
    ('Setting',                'settings',  '/setting',                 30),
    ('MCU',                    'heart',     '/mcu',                     40),
    ('Report',                 'chart',     '/report',                  50),
    ('Master Data Klinik',     'database',  '/master-data-klinik',      60),
    ('Front Office Klinik',    'monitor',   '/front-office-klinik',     70),
    ('Transaksi Klinik',       'boxes',     '/transaksi-klinik',        80),
    ('Report Klinik',          'bar-chart', '/report-klinik',           90),
    ('Stok Farmasi',           'monitor',   '/stok-farmasi',           100),
    ('Upload',                 'upload',    '/upload',                 110),
    ('Management',             'settings',  '/management',             120),
    ('Screening Covid 19',     'database',  '/screening-covid-19',     130),
    ('Medical Check Up',       'heart',     '/medical-check-up',       140),
    ('Master Data Satu Sehat', 'database',  '/master-data-satu-sehat', 150),
    ('Report Satu Sehat',      'database',  '/report-satu-sehat',      160);

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('MCU',            'circle', '/master-data/mcu',            10),
    ('Perusahaan',     'users',  '/master-data/perusahaan',     20),
    ('Pasien',         'user',   '/master-data/pasien',         30),
    ('Lab Master',     'circle', '/master-data/lab-master',     40),
    ('Lab Sub Master', 'circle', '/master-data/lab-sub-master', 50),
    ('Lab',            'circle', '/master-data/lab',            60),
    ('Non Lab',        'circle', '/master-data/non-lab',        70),
    ('Dokter Master',  'user',   '/master-data/dokter-master',  80),
    ('Dokter',         'user',   '/master-data/dokter',         90)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Master Data' AND p.parent_id IS NULL;
