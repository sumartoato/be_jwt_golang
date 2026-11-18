#!/usr/bin/env bash
# Copies patient registrations (trxpendaftaran), services (refjenisrawat),
# visit statuses (refstatustransaksi) and patient point balances
# (refpasienpoint) from the legacy "lab" database. Needs the patients first
# (scripts/import-lab-pasien.sh).
#
# The data is personal (who visited which clinic when): keep it on machines
# allowed to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Point balances of
# patients that do not exist in the app are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxpendaftaran "$REPLACE"

echo "Clearing app registration tables..."
app -c "TRUNCATE trxpendaftaran, refpasienpoint, refjenisrawat, refstatustransaksi RESTART IDENTITY CASCADE"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisrawat      "id, info, ispemeriksaan, isactive, idnpwp, neednoantrian, firstcode"
copy refstatustransaksi "id, info"
# An empty branch code (194 rows in lab) becomes NULL so the branch foreign key holds.
copy_query trxpendaftaran "id, no_antri, tgl_masuk, tgl_keluar, id_pas, id_jenis_rwt, id_statustr, create_user, create_date,
                     update_user, update_date, cabang, status_kartu, cancelby, cancelat, cancelreason, no_antri_poli,
                     no_antri_poli_str" \
  "SELECT id, no_antri, tgl_masuk, tgl_keluar, id_pas, id_jenis_rwt, id_statustr, create_user, create_date, update_user,
          update_date, NULLIF(cabang, ''), status_kartu, cancelby, cancelat, cancelreason, no_antri_poli, no_antri_poli_str
   FROM trxpendaftaran"

# Point balances only for patients the app has.
app -c "CREATE TABLE IF NOT EXISTS import_pasienpoint (id_pas int, point double precision); TRUNCATE import_pasienpoint"
lab -c "\\copy (SELECT id_pas, point FROM refpasienpoint) TO STDOUT" | app -c "\\copy import_pasienpoint FROM STDIN"
app -At -c "INSERT INTO refpasienpoint (id_pas, point) SELECT i.id_pas, i.point FROM import_pasienpoint i JOIN refmpasien p ON p.id = i.id_pas;
            SELECT format('  %-20s %s of %s rows (rest: patients not in the app)', 'refpasienpoint',
                          (SELECT count(*) FROM refpasienpoint), (SELECT count(*) FROM import_pasienpoint));
            DROP TABLE import_pasienpoint;"

reset_identity refjenisrawat refstatustransaksi trxpendaftaran
app -c "ANALYZE trxpendaftaran" >/dev/null
echo "Done."
