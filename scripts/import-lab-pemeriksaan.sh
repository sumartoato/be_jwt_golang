#!/usr/bin/env bash
# Copies the examination notes per registration (trxpemeriksaan) from the
# legacy "lab" database. They hold the responsible doctor ("Dokter PJ") that the
# Pengobatan form shows when picking a registration.
#
# The data is medical (complaints, findings, vital signs): keep it on machines
# allowed to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pemeriksaan.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-pemeriksaan.sh --replace  # wipe and copy again
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxpemeriksaan "$REPLACE"

echo "Clearing app examination table..."
app -c "TRUNCATE trxpemeriksaan RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy trxpemeriksaan "id, id_kun, keluhan_utama, anamnesis, tekanan_darah_sistole, tekanan_darah_diastole, berat_badan,
                     tinggi_badan, pemeriksaanfisik, rekap_hasil_pemeriksaan_penunjang, hasil_pemeriksaan_penunjang,
                     rencana_penatalaksanaan, create_user, create_date, update_user, update_date, suhu, id_dok,
                     hasil_pemeriksaan_dokter, tindakan_dokter"

reset_identity trxpemeriksaan
app -c "ANALYZE trxpemeriksaan" >/dev/null
echo "Done."
