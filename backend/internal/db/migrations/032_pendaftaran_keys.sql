-- Keys for the registration tables of 031, checked against the lab data.
--   * service, visit status and branch references are all valid (after the
--     import turns 194 empty branch codes into NULL);
--   * id_pas has no foreign key: 38,174 lab registrations point to patients
--     that no longer exist, and they are kept as visit history;
--   * queue numbers are not unique (lab has 9 repeats within a branch and day).
ALTER TABLE trxpendaftaran
    ADD CONSTRAINT fk_trxpendaftaran_jenis  FOREIGN KEY (id_jenis_rwt) REFERENCES refjenisrawat (id),
    ADD CONSTRAINT fk_trxpendaftaran_status FOREIGN KEY (id_statustr)  REFERENCES refstatustransaksi (id),
    ADD CONSTRAINT fk_trxpendaftaran_cabang FOREIGN KEY (cabang)       REFERENCES refbranch (kode);
