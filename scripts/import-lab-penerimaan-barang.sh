#!/usr/bin/env bash
# Copies goods receipts (trxfaktur), their drug lines (trxfakturobat), totals
# (trxfakturtotal) and the suppliers (refsupplier) from the legacy "lab"
# database, for Transaksi Klinik > Penerimaan Barang.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-penerimaan-barang.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-penerimaan-barang.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Lines and totals
# whose receipt does not exist in lab are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxfaktur "$REPLACE"

echo "Clearing app goods receipt tables..."
app -c "TRUNCATE trxfakturtotal, trxfakturobat, trxfaktur, refsupplier RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refsupplier "id, nama, alamat, telp, fax, cp, telp_cp, create_user, create_date, update_user, update_date"
copy_query trxfaktur "id, tgl, tgl_jtp, no_fak, tgl_fak, id_supp, id_jenis_byr, create_user, create_date, update_user, update_date,
                      cabang, id_distributor" \
  "SELECT id, tgl, tgl_jtp, no_fak, tgl_fak, id_supp, id_jenis_byr, create_user, create_date, update_user, update_date,
          NULLIF(cabang, ''), id_distributor
   FROM trxfaktur"
copy_query trxfakturobat "id, id_fak, id_obat, no_batch, tgl_exp, disc, hbeli, byk, tot_qjb" \
  "SELECT o.id, o.id_fak, o.id_obat, o.no_batch, o.tgl_exp, o.disc, o.hbeli, o.byk, o.tot_qjb
   FROM trxfakturobat o WHERE EXISTS (SELECT 1 FROM trxfaktur f WHERE f.id = o.id_fak)"
copy_query trxfakturtotal "id_fak, total, cicilan, potongan, utang, no_close, tgl_close, status" \
  "SELECT t.id_fak, t.total, t.cicilan, t.potongan, t.utang, t.no_close, t.tgl_close, t.status
   FROM trxfakturtotal t WHERE EXISTS (SELECT 1 FROM trxfaktur f WHERE f.id = t.id_fak)"

reset_identity refsupplier trxfaktur trxfakturobat
app -c "ANALYZE trxfaktur; ANALYZE trxfakturobat; ANALYZE trxfakturtotal" >/dev/null
echo "Done."
