-- Top-level "MCU" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
-- parent_id IS NULL matters: there is also an "MCU" item under Master Data.
UPDATE menus SET path = NULL WHERE title = 'MCU' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, 'circle', c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Pendaftaran',                '/mcu/pendaftaran',                 10),
    ('Konfirmasi Kehadiran',       '/mcu/konfirmasi-kehadiran',        20),
    ('Mulai Pemeriksaan',          '/mcu/mulai-pemeriksaan',           30),
    ('Pemeriksaan',                '/mcu/pemeriksaan',                 40),
    ('Validasi Hasil',             '/mcu/validasi-hasil',              50),
    ('Resume',                     '/mcu/resume',                      60),
    ('Close Dokumen',              '/mcu/close-dokumen',               70),
    ('Search Data',                '/mcu/search-data',                 80),
    ('Pemeriksaan Fisik',          '/mcu/pemeriksaan-fisik',           90),
    ('Riwayat Medis',              '/mcu/riwayat-medis',              100),
    ('Pemeriksaan Lab',            '/mcu/pemeriksaan-lab',            110),
    ('Pemeriksaan Audiometri',     '/mcu/pemeriksaan-audiometri',     120),
    ('Riwayat Kesehatan Keluarga', '/mcu/riwayat-kesehatan-keluarga', 130),
    ('Riwayat Khusus wanita',      '/mcu/riwayat-khusus-wanita',      140),
    ('Riwayat Opname & Operasi',   '/mcu/riwayat-opname-operasi',     150),
    ('Kebiasaan Pasien',           '/mcu/kebiasaan-pasien',           160),
    ('Riwayat Pekerjaan',          '/mcu/riwayat-pekerjaan',          170),
    ('Anamnesa',                   '/mcu/anamnesa',                   180),
    ('Pemeriksaan Gigi',           '/mcu/pemeriksaan-gigi',           190),
    ('Rontgen Thorax',             '/mcu/rontgen-thorax',             200),
    ('EKG',                        '/mcu/ekg',                        210),
    ('EKG TreadMill',              '/mcu/ekg-treadmill',              220),
    ('Spirometry',                 '/mcu/spirometry',                 230),
    ('USG',                        '/mcu/usg',                        240)
) AS c (title, path, sort_order)
WHERE p.title = 'MCU' AND p.parent_id IS NULL;
