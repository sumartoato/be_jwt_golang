-- "Report Klinik" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Report Klinik' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Raw Data Pengobatan', 'file',   '/report-klinik/raw-data-pengobatan',  10),
    ('Penjualan Obat',      'file',   '/report-klinik/penjualan-obat',       20),
    ('Penjualan Tindakan',  'file',   '/report-klinik/penjualan-tindakan',   30),
    ('Penjualan Penunjang', 'file',   '/report-klinik/penjualan-penunjang',  40),
    ('Penjualan OTC',       'file',   '/report-klinik/penjualan-otc',        50),
    ('Pasien Baru',         'file',   '/report-klinik/pasien-baru',          60),
    ('Rekap Asuransi',      'file',   '/report-klinik/rekap-asuransi',       70),
    ('Transaksi Harian',    'file',   '/report-klinik/transaksi-harian',     80),
    ('All Stok',            'file',   '/report-klinik/all-stok',             90),
    ('Rujukan',             'file',   '/report-klinik/rujukan',             100),
    ('Penjualan Softdrink', 'file',   '/report-klinik/penjualan-softdrink', 110),
    ('Pendaftaran',         'file',   '/report-klinik/pendaftaran',         120),
    ('Report Pendapatan',   'file',   '/report-klinik/report-pendapatan',   130),
    ('Report Diagnosa',     'file',   '/report-klinik/report-diagnosa',     140),
    ('Report ICD Dinkes',   'circle', '/report-klinik/report-icd-dinkes',   150)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Report Klinik' AND p.parent_id IS NULL;
