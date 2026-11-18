#!/usr/bin/env bash
# Copies doctors (refmdokter + refjenisdokter), the branches they practise at
# (refmdokterauth) and the procedures they may perform (trxtindakan) from the
# legacy "lab" database. Needs the patient lookups (gender, agama, ...) and
# reftindakan already imported (scripts/import-lab-pasien.sh, import-lab-tindakan.sh).
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-dokter.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-dokter.sh --replace  # wipe app doctor data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Mappings that point to
# a missing doctor, procedure or branch are skipped, and duplicates are imported once.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refmdokter "$REPLACE"

echo "Clearing app doctor tables..."
app -c "TRUNCATE trxtindakan, refmdokterauth, refmdokter, refjenisdokter RESTART IDENTITY CASCADE"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisdokter "id, info"
copy refmdokter "id, nama, id_jenisdok, tgl_masuk, tmpt_lahir, tgl_lahir, alamat, telp, hp, pendidikan, spesialis,
                 id_gender, id_agama, id_goldarah, id_statuskwn, create_user, create_date, update_user, update_date,
                 id_internal, cabang, oldid, status, nik, ihs_id, ihs_name, ihs_status, notesatusehat"

# Load the mappings into staging tables first, then keep only rows whose
# doctor, procedure and branch exist in the app, once per pair.
app -c "CREATE TABLE IF NOT EXISTS import_dokterauth (id int, dokterid int, cabang varchar(10));
        CREATE TABLE IF NOT EXISTS import_trxtindakan (id int, id_dokter int, id_tindakan int);
        TRUNCATE import_dokterauth, import_trxtindakan"
lab -c "\\copy (SELECT id, dokterid, cabang FROM refmdokterauth) TO STDOUT" | app -c "\\copy import_dokterauth FROM STDIN"
lab -c "\\copy (SELECT id, id_dokter, id_tindakan FROM trxtindakan) TO STDOUT" | app -c "\\copy import_trxtindakan FROM STDIN"
app -At <<'SQL'
INSERT INTO refmdokterauth (id, dokterid, cabang)
SELECT DISTINCT ON (a.dokterid, a.cabang) a.id, a.dokterid, a.cabang
FROM import_dokterauth a
JOIN refmdokter d ON d.id = a.dokterid
JOIN refbranch b ON b.kode = a.cabang
ORDER BY a.dokterid, a.cabang, a.id;

INSERT INTO trxtindakan (id, id_dokter, id_tindakan)
SELECT DISTINCT ON (t.id_dokter, t.id_tindakan) t.id, t.id_dokter, t.id_tindakan
FROM import_trxtindakan t
JOIN refmdokter d ON d.id = t.id_dokter
JOIN reftindakan r ON r.id = t.id_tindakan
ORDER BY t.id_dokter, t.id_tindakan, t.id;

SELECT format('  %-20s %s of %s rows (rest: missing doctor/branch or duplicates)', 'refmdokterauth',
              (SELECT count(*) FROM refmdokterauth), (SELECT count(*) FROM import_dokterauth));
SELECT format('  %-20s %s of %s rows (rest: missing doctor/procedure or duplicates)', 'trxtindakan',
              (SELECT count(*) FROM trxtindakan), (SELECT count(*) FROM import_trxtindakan));
DROP TABLE import_dokterauth, import_trxtindakan;
SQL

reset_identity refjenisdokter refmdokter refmdokterauth trxtindakan
echo "Done."
