#!/usr/bin/env bash
# Copies the lines of the treatment sales from the legacy "lab" database: drugs
# (trxjualobat), procedures (trxjualtindakan), supporting examinations
# (trxjualpnjng), diagnoses (trxjualdiagnosa) and receipt prints (trxjualcetak). Needs the sales first
# (scripts/import-lab-pengobatan.sh); lines whose sale does not exist in the
# app are skipped (104 in lab).
#
# The data is medical (what was prescribed and diagnosed): keep it on machines
# allowed to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pengobatan-detail.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-pengobatan-detail.sh --replace  # wipe and copy again
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxjualobat "$REPLACE"

echo "Clearing app sale line tables..."
app -c "TRUNCATE trxjualobat, trxjualtindakan, trxjualpnjng, trxjualdiagnosa, trxjualcetak RESTART IDENTITY"

# copy_lines TABLE "COLUMNS": copy into a staging table, then keep the lines of known sales.
copy_lines() {
  local table=$1 cols=$2
  app -c "SET client_min_messages = warning; DROP TABLE IF EXISTS import_lines; CREATE TABLE import_lines (LIKE $table)" >/dev/null
  lab -c "\\copy (SELECT $cols FROM $table) TO STDOUT" | app -c "\\copy import_lines ($cols) FROM STDIN"
  app -At -c "INSERT INTO $table ($cols) OVERRIDING SYSTEM VALUE SELECT $cols FROM import_lines i WHERE EXISTS (SELECT 1 FROM trxjual j WHERE j.id = i.id_jual);
              SELECT format('  %-18s %s of %s rows (rest: sales not in the app)', '$table', (SELECT count(*) FROM $table), (SELECT count(*) FROM import_lines));
              DROP TABLE import_lines;"
}

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy_lines trxjualobat     "id, id_jual, id_obat, hjual, byk, hp, dosis, ppn"
copy_lines trxjualtindakan "id, id_jual, id_tndk, hjual, byk, id_dok"
copy_lines trxjualpnjng    "id, id_jual, id_pnjng, hjual, byk"
copy_lines trxjualdiagnosa "id, id_jual, icd_id"

# Receipt prints: one per sale, no id column.
app -c "SET client_min_messages = warning; DROP TABLE IF EXISTS import_cetak; CREATE TABLE import_cetak (LIKE trxjualcetak)" >/dev/null
lab -c "\copy (SELECT id_jual, no_kwi, no_cetak, cabang, userid, createdate FROM trxjualcetak) TO STDOUT" | app -c "\copy import_cetak FROM STDIN"
app -At -c "INSERT INTO trxjualcetak SELECT i.* FROM import_cetak i WHERE EXISTS (SELECT 1 FROM trxjual j WHERE j.id = i.id_jual);
            SELECT format('  %-18s %s of %s rows (rest: sales not in the app)', 'trxjualcetak', (SELECT count(*) FROM trxjualcetak), (SELECT count(*) FROM import_cetak));
            DROP TABLE import_cetak;"

reset_identity trxjualobat trxjualtindakan trxjualpnjng trxjualdiagnosa
app -c "ANALYZE trxjualobat; ANALYZE trxjualtindakan; ANALYZE trxjualpnjng; ANALYZE trxjualdiagnosa" >/dev/null
echo "Done."
