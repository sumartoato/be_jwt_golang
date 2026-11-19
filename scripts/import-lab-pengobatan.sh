#!/usr/bin/env bash
# Copies treatment sales (trxjual), their totals (trxjualtotal) and the payment
# types (refjenisbayar) from the legacy "lab" database. The list also shows
# patients, doctors and services, so import Pasien, Dokter and Pendaftaran first.
#
# The data is personal (who was treated where and paid what): keep it on
# machines allowed to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pengobatan.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-pengobatan.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Totals whose sale does
# not exist in lab (43 rows) are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxjual "$REPLACE"

echo "Clearing app sales tables..."
app -c "TRUNCATE trxjualtotal, trxjual, refjenisbayar RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisbayar "id, info, flag"
copy_query trxjual "id, tgl, no_kwi, tgl_kwi, id_pas, id_jenis_byr, id_kun, id_jenis_rwt, id_dok, create_user, create_date,
                    update_user, update_date, id_perujuk_otc, id_jamkes, no_jamkes, cabang, flow_status, encounterid,
                    encounterstatus, encountercreateddate, encounterupdatedate, encounterfinisheddate" \
  "SELECT id, tgl, no_kwi, tgl_kwi, id_pas, id_jenis_byr, id_kun, id_jenis_rwt, id_dok, create_user, create_date,
          update_user, update_date, id_perujuk_otc, id_jamkes, no_jamkes, NULLIF(cabang, ''), flow_status, encounterid,
          encounterstatus, encountercreateddate, encounterupdatedate, encounterfinisheddate
   FROM trxjual"
copy_query trxjualtotal "id_jual, tot_obat, tot_tndk, tot_pnjng, total, cicilan, potongan, piutang, no_close, tgl_close,
                         status, point, pembayaran, cancelby, cancelat, cancelreason" \
  "SELECT t.id_jual, t.tot_obat, t.tot_tndk, t.tot_pnjng, t.total, t.cicilan, t.potongan, t.piutang, t.no_close,
          t.tgl_close, t.status, t.point, t.pembayaran, t.cancelby, t.cancelat, t.cancelreason
   FROM trxjualtotal t WHERE EXISTS (SELECT 1 FROM trxjual j WHERE j.id = t.id_jual)"

reset_identity refjenisbayar trxjual
app -c "ANALYZE trxjual; ANALYZE trxjualtotal" >/dev/null
echo "Done."
