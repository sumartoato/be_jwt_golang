-- "Transaksi Klinik" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Transaksi Klinik' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, 'user', c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Pengobatan',          '/transaksi-klinik/pengobatan',          10),
    ('Faktur',              '/transaksi-klinik/faktur',              20),
    ('Penerimaan Barang',   '/transaksi-klinik/penerimaan-barang',   30),
    ('Pemeriksaan Perawat', '/transaksi-klinik/pemeriksaan-perawat', 40),
    ('Pemeriksaan Dokter',  '/transaksi-klinik/pemeriksaan-dokter',  50),
    ('Proses Obat',         '/transaksi-klinik/proses-obat',         60)
) AS c (title, path, sort_order)
WHERE p.title = 'Transaksi Klinik' AND p.parent_id IS NULL;
