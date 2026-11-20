-- Receipt prints (trxjualcetak): one row per sale, written when its receipt is
-- printed the first time. Same columns and primary key as in the legacy "lab"
-- database; loaded with scripts/import-lab-pengobatan-detail.sh, which skips
-- the 198,871 lab rows whose sale no longer exists.
--   no_cetak  <branch receipt code>-YY-MM-<5 digits>, numbered per code and month,
--             e.g. CTM-26-11-00320; the code is refbranch.kode_kwt (MED prints BTA)
--   userid    who printed it; createdate  when (clinic time)
CREATE TABLE IF NOT EXISTS trxjualcetak (
    id_jual    integer     NOT NULL,
    no_kwi     varchar(11) NOT NULL,
    no_cetak   varchar(15) NOT NULL,
    cabang     varchar(10),
    userid     varchar(50),
    createdate timestamp(3),
    CONSTRAINT trxjualcetak_pkey PRIMARY KEY (id_jual),
    CONSTRAINT fk_trxjualcetak_jual FOREIGN KEY (id_jual) REFERENCES trxjual (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS trxjualcetak_no_cetak_idx ON trxjualcetak (no_cetak);
