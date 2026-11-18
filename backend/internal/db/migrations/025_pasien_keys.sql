-- Keys for the patient tables of 024. Checked against the lab data: every
-- reference below is valid (after the import turns 19 empty branch codes into
-- NULL), and the lookup ids are unique.

-- Primary keys that the lab tables lack.
ALTER TABLE refgender      ADD CONSTRAINT refgender_pkey      PRIMARY KEY (id);
ALTER TABLE refagama       ADD CONSTRAINT refagama_pkey       PRIMARY KEY (id);
ALTER TABLE refstatuskawin ADD CONSTRAINT refstatuskawin_pkey PRIMARY KEY (id);
ALTER TABLE refkelurahan   ADD CONSTRAINT refkelurahan_pkey   PRIMARY KEY (id);

-- Region hierarchy.
ALTER TABLE refkelurahan ADD CONSTRAINT fk_refkelurahan_kec FOREIGN KEY (id_kec)  REFERENCES refkecamatan (id);
ALTER TABLE refkecamatan ADD CONSTRAINT fk_refkecamatan_kab FOREIGN KEY (id_kab)  REFERENCES refkabupaten (id);
ALTER TABLE refkabupaten ADD CONSTRAINT fk_refkabupaten_prov FOREIGN KEY (id_prov) REFERENCES refprovinsi (id);

-- Patient references. No RM is not unique (it repeats across branches and
-- even within one in lab), so it gets no unique constraint.
ALTER TABLE refmpasien
    ADD CONSTRAINT fk_refmpasien_kel       FOREIGN KEY (id_kel)        REFERENCES refkelurahan (id),
    ADD CONSTRAINT fk_refmpasien_gender    FOREIGN KEY (id_gender)     REFERENCES refgender (id),
    ADD CONSTRAINT fk_refmpasien_goldarah  FOREIGN KEY (id_goldarah)   REFERENCES refgoldarah (id),
    ADD CONSTRAINT fk_refmpasien_agama     FOREIGN KEY (id_agama)      REFERENCES refagama (id),
    ADD CONSTRAINT fk_refmpasien_statuskwn FOREIGN KEY (id_status_kwn) REFERENCES refstatuskawin (id),
    ADD CONSTRAINT fk_refmpasien_jamkes    FOREIGN KEY (id_jamkes)     REFERENCES refjaminankesehatan (id),
    ADD CONSTRAINT fk_refmpasien_cabang    FOREIGN KEY (cabang)        REFERENCES refbranch (kode);
CREATE INDEX IF NOT EXISTS refmpasien_id_kel_idx ON refmpasien (id_kel);
