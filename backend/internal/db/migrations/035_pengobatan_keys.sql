-- Keys for the sales tables of 034, checked against the lab data.
--   * payment types are all valid, and receipt numbers are present and unique;
--   * every imported total belongs to a sale (the import skips 43 orphans);
--     425 sales have no total and show as Open with 0;
--   * cabang has no foreign key: 355 old sales use codes no branch has
--     (MED_, MGM_, MIT_, PRM_). Branch authorization hides them, as in lab;
--   * no foreign keys to patients, doctors, services or registrations: lab has
--     7 sales of deleted patients, 1 of a deleted doctor and 112 of deleted
--     registrations, and the Pasien, Dokter and Pendaftaran re-imports truncate
--     those tables (Dokter and Pendaftaran with CASCADE, which would empty
--     these tables too).
ALTER TABLE trxjual
    ADD CONSTRAINT fk_trxjual_jenis_byr FOREIGN KEY (id_jenis_byr) REFERENCES refjenisbayar (id),
    ADD CONSTRAINT uq_trxjual_no_kwi    UNIQUE (no_kwi);

ALTER TABLE trxjualtotal
    ADD CONSTRAINT fk_trxjualtotal_jual FOREIGN KEY (id_jual) REFERENCES trxjual (id) ON DELETE CASCADE;
