#!/usr/bin/env bash
# Copies patients (refmpasien) and the tables they refer to (gender, blood
# type, religion, marital status, insurers and the region hierarchy) from the
# legacy "lab" database into the app database.
#
# The data is personal and medical: keep this copy on machines that are allowed
# to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pasien.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-pasien.sh --replace  # wipe app patient data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refmpasien "$REPLACE"

echo "Clearing app patient tables..."
app -c "TRUNCATE refmpasien, refgender, refgoldarah, refagama, refstatuskawin, refjaminankesehatan,
        refkelurahan, refkecamatan, refkabupaten, refprovinsi RESTART IDENTITY CASCADE"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refgender       "id, info"
copy refgoldarah     "id, info"
copy refagama        "id, info"
copy refstatuskawin  "id, info"
copy refjaminankesehatan 'id, nama, alamat, telp, fax, create_user, create_date, update_user, update_date, isactive, jamkestype, status_limit, "limit"'
copy refprovinsi     "id, info"
copy refkabupaten    "id, info, id_prov"
copy refkecamatan    "id, info, id_kab"
copy refkelurahan    "id, info, id_kec, kode_kemendagri"
# An empty branch code ('', 19 patients in lab) becomes NULL so the branch foreign key holds.
cols="id, no_rm, nama, tmpt_lahir, tgl_lahir, alamat, id_kel, telp, hp, id_gender, id_goldarah, id_agama,
      id_status_kwn, tgl_kun, penanggung, id_jamkes, create_user, create_date, update_user, update_date,
      no_jamkes, cabang, nik, alergi, ihs_id, ihs_name, ihs_status, notesatusehat, rt, rw"
copy_query refmpasien "$cols" "SELECT id, no_rm, nama, tmpt_lahir, tgl_lahir, alamat, id_kel, telp, hp, id_gender,
  id_goldarah, id_agama, id_status_kwn, tgl_kun, penanggung, id_jamkes, create_user, create_date, update_user,
  update_date, no_jamkes, NULLIF(cabang, ''), nik, alergi, ihs_id, ihs_name, ihs_status, notesatusehat, rt, rw
  FROM refmpasien"

reset_identity refgender refgoldarah refagama refstatuskawin refjaminankesehatan refprovinsi refkabupaten \
  refkecamatan refkelurahan refmpasien
app -c "ANALYZE refmpasien" >/dev/null
echo "Done."
