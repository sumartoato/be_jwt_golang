-- Clinic branches (Management > Cabang) with their Satu Sehat settings.
--
--   code             short branch code, stored uppercase, unique (e.g. CTM)
--   fasyankes_*      the facility as registered at the Ministry of Health
--   client_key,
--   secret_key       Satu Sehat OAuth credentials. The API never returns secret_key,
--                    only whether it is set and its last 4 characters
--   organization_id  Satu Sehat Organization ID
--   db_hbee          name of the branch's HBee database
CREATE TABLE IF NOT EXISTS branches (
    id              SERIAL PRIMARY KEY,
    code            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    fasyankes_code  TEXT NOT NULL DEFAULT '',
    fasyankes_name  TEXT NOT NULL DEFAULT '',
    client_key      TEXT NOT NULL DEFAULT '',
    secret_key      TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    db_hbee         TEXT NOT NULL DEFAULT '',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Keys are not seeded: they are credentials and are entered in the app.
INSERT INTO branches (code, name, fasyankes_code, fasyankes_name, organization_id, db_hbee) VALUES
    ('CTM', 'KLINIK CITRA MEDIKA',                            '31710300339', 'KLINIK PRATAMA CITRA MEDIKA',                          '100468100', ''),
    ('DJP', 'KLINIK DIREKTORAT JENDRAL PAJAK',                '31740300614', 'Klinik Pratama Kantor Pusat Direktorat Jenderal Pajak', '100468474', ''),
    ('JMA', 'KLINIK JMA SYARIAH',                             '',            'KLINIK JMA SYARIAH',                                   '',          ''),
    ('MED', 'KLINIK MEDIKA',                                  '31750300209', 'Klinik Pratama Medika',                                '100018337', 't79144_medika'),
    ('MGM', 'KLINIK MGM 24',                                  '31750300259', 'Klinik MGM',                                           '100018312', 't79144_x'),
    ('MIT', 'KLINIK MITRA CAKUNG',                            '31750300131', 'Klinik mitra cakung',                                  '100018213', 't79144_labcakung'),
    ('KMJ', 'KLINIK MITRA JATINEGARA',                        '303',         'KLINIK MITRA JATINEGARA',                              '101082707', ''),
    ('KJI', 'KLINIK MITRA KRAMAT JATI',                       '',            '',                                                     '',          't79144_medika'),
    ('KMM', 'KLINIK MITRA MEDIKA',                            '31720300396', 'KLINIK PRATAMA MITRA MEDIKA',                          '100467471', 't79144_x'),
    ('PRM', 'KLINIK PRATAMA PRIMA MEDIKA',                    '31750300503', 'Klinik Pratama Prima Medika',                          '100483644', 't79144_x'),
    ('OHC', 'KLINIK UTAMA MEDIKA OCCUPATIONAL HEALTH CENTER', '31750300106', 'Klinik Utama Medika Occupational Health Center',       '100018354', '');
