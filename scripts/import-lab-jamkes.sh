#!/usr/bin/env bash
# Copies the insurers' PKS terms (refjaminankesehatan_detail) and item lists
# (refjaminankesehatan_detail_item) from the legacy "lab" database. The insurers
# themselves come with scripts/import-lab-pasien.sh (re-running that import
# empties these two tables, so run this one again afterwards).
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-jamkes.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-jamkes.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refjaminankesehatan_detail "$REPLACE"

echo "Clearing app insurer detail tables..."
app -c "TRUNCATE refjaminankesehatan_detail, refjaminankesehatan_detail_item RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjaminankesehatan_detail      "id, jamkesid, detailid, value, flag"
copy refjaminankesehatan_detail_item "id, jamkesid, detailid, value, flag"

reset_identity refjaminankesehatan_detail refjaminankesehatan_detail_item
echo "Done."
