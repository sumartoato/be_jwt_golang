-- Keys for the doctor tables of 026. Checked against the lab data: every
-- reference is valid; trxtindakan has 1,512 duplicate doctor/procedure pairs,
-- which the import loads once.

ALTER TABLE refmdokter
    ADD CONSTRAINT fk_refmdokter_jenis     FOREIGN KEY (id_jenisdok)  REFERENCES refjenisdokter (id),
    ADD CONSTRAINT fk_refmdokter_gender    FOREIGN KEY (id_gender)    REFERENCES refgender (id),
    ADD CONSTRAINT fk_refmdokter_agama     FOREIGN KEY (id_agama)     REFERENCES refagama (id),
    ADD CONSTRAINT fk_refmdokter_goldarah  FOREIGN KEY (id_goldarah)  REFERENCES refgoldarah (id),
    ADD CONSTRAINT fk_refmdokter_statuskwn FOREIGN KEY (id_statuskwn) REFERENCES refstatuskawin (id);

-- One row per doctor and branch / doctor and procedure.
ALTER TABLE refmdokterauth
    ADD CONSTRAINT refmdokterauth_dokter_cabang_key UNIQUE (dokterid, cabang),
    ADD CONSTRAINT fk_refmdokterauth_dokter FOREIGN KEY (dokterid) REFERENCES refmdokter (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_refmdokterauth_cabang FOREIGN KEY (cabang) REFERENCES refbranch (kode);
ALTER TABLE trxtindakan
    ADD CONSTRAINT trxtindakan_dokter_tindakan_key UNIQUE (id_dokter, id_tindakan),
    ADD CONSTRAINT fk_trxtindakan_dokter FOREIGN KEY (id_dokter) REFERENCES refmdokter (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_trxtindakan_tindakan FOREIGN KEY (id_tindakan) REFERENCES reftindakan (id) ON DELETE CASCADE;
