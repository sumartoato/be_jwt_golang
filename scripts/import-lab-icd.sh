#!/usr/bin/env bash
# Copies ICD-X codes (reficd) and their types (refjenisicd) from the legacy
# "lab" database into the app database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-icd.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-icd.sh --replace  # wipe app ICD data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reficd "$REPLACE"

echo "Clearing app ICD tables..."
app -c "TRUNCATE reficd, refjenisicd RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisicd "id, info"
copy reficd "id, kode_icd, nama, id_jenis_icd, create_user, create_date, update_user, update_date"

reset_identity refjenisicd reficd
app -c "ANALYZE reficd" >/dev/null
echo "Done."
