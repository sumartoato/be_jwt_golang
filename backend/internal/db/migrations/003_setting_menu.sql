-- "Setting" becomes a group with its own submenu. Runs once (tracked in schema_migrations).
UPDATE menus SET path = NULL WHERE title = 'Setting' AND parent_id IS NULL;

INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, c.title, c.icon, c.path, c.sort_order
FROM menus p
CROSS JOIN (VALUES
    ('Price',                      'circle',  '/setting/price',                    10),
    ('Paket Price',                'circle',  '/setting/paket-price',              20),
    ('Barcode',                    'barcode', '/setting/barcode',                  30),
    ('Normal Abnormal Pem. Fisik', 'circle',  '/setting/normal-abnormal-pem-fisik', 40),
    ('Normal Abnormal Pem. Lab',   'circle',  '/setting/normal-abnormal-pem-lab',   50),
    ('Saran',                      'circle',  '/setting/saran',                    60),
    ('Kategori',                   'circle',  '/setting/kategori',                 70)
) AS c (title, icon, path, sort_order)
WHERE p.title = 'Setting' AND p.parent_id IS NULL;
