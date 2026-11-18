#!/usr/bin/env bash
# Copies the SATUSEHAT organizations and locations per branch and poli
# (reforganizationlocation) from the legacy "lab" database. The poli names come
# from refjenisrawat, loaded by scripts/import-lab-pendaftaran.sh.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Rows of branches the
# app does not have are skipped; empty SATUSEHAT ids become NULL.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reforganizationlocation "$REPLACE"

echo "Clearing app organizations & locations..."
app -c "TRUNCATE reforganizationlocation RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy_query reforganizationlocation \
  "id, cabang, jenisrawatid, organizationname, locationname, organizationkeyid, locationkeyid, updatedby, updatedat" \
  "SELECT id, cabang, jenisrawatid, organizationname, locationname, NULLIF(organizationkeyid, ''),
          NULLIF(locationkeyid, ''), updatedby, updatedat
   FROM reforganizationlocation WHERE cabang IN ($(app_branches))"
reset_identity reforganizationlocation

if [[ "$(app -At -c "SELECT count(*) FROM refjenisrawat")" == "0" ]]; then
  echo "Note: refjenisrawat is empty, so the Poli column stays blank. Run scripts/import-lab-pendaftaran.sh." >&2
fi
echo "Done."
