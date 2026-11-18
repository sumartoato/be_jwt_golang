#!/usr/bin/env bash
# Copies supporting examinations (refpenunjang + refjenispanjang) and their
# prices per branch (refpenunjangdetail) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-penunjang.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-penunjang.sh --replace  # wipe app penunjang data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Price rows for
# branches missing from the app are skipped; duplicates are imported once.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refpenunjang "$REPLACE"

echo "Clearing app penunjang tables..."
app -c "TRUNCATE refpenunjangdetail, refpenunjang, refjenispanjang RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenispanjang "id, info"
copy refpenunjang "id, nama, id_jenis, hjual, create_user, create_date, update_user, update_date, isactive, hjual_jamkes"

cols="id, id_penunjang, cabang, hjual, hjual_jamkes, isactive"
copy_query refpenunjangdetail "$cols" \
  "SELECT DISTINCT ON (id_penunjang, cabang) $cols FROM refpenunjangdetail
   WHERE cabang IN ($(app_branches)) ORDER BY id_penunjang, cabang, id"

reset_identity refjenispanjang refpenunjang refpenunjangdetail
echo "Done."
