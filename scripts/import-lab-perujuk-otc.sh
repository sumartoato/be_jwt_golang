#!/usr/bin/env bash
# Copies the OTC referrers (refperujukotc) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refperujukotc "$REPLACE"

echo "Clearing app OTC referrers..."
app -c "TRUNCATE refperujukotc RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refperujukotc "id, nama"
reset_identity refperujukotc
echo "Done."
