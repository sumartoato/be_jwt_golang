#!/usr/bin/env bash
# Copies the drug master (refobat + lookups) and stock per branch (trxstock)
# from the legacy "lab" database into the app database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-obat.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-obat.sh --replace  # wipe app drug data first
#
# Optional: LAB_HOST (default host.docker.internal, i.e. the Docker host),
# LAB_PORT (5432), LAB_DB (lab), LAB_USER (postgres).
#
# Rows are read with \copy, so it works across PostgreSQL versions. Stock rows
# for drugs that no longer exist in lab, or for branches missing from the app's
# refbranch, are skipped.
set -euo pipefail

: "${LAB_PASSWORD:?Set LAB_PASSWORD to the password of the lab database}"
LAB_HOST="${LAB_HOST:-host.docker.internal}"
LAB_PORT="${LAB_PORT:-5432}"
LAB_DB="${LAB_DB:-lab}"
LAB_USER="${LAB_USER:-postgres}"
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

cd "$(dirname "$0")/.."
export MSYS_NO_PATHCONV=1 # Git Bash on Windows: do not rewrite arguments

lab() { docker compose exec -T -e PGPASSWORD="$LAB_PASSWORD" db psql -X -q -v ON_ERROR_STOP=1 -h "$LAB_HOST" -p "$LAB_PORT" -U "$LAB_USER" -d "$LAB_DB" "$@"; }
app() { docker compose exec -T db sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"' psql "$@"; }

existing=$(app -At -c "SELECT count(*) FROM refobat")
if [[ "$existing" != "0" && "$REPLACE" != "1" ]]; then
  echo "refobat already has $existing rows. Re-run with --replace to wipe and re-import." >&2
  exit 1
fi

echo "Clearing app drug tables..."
app -c "TRUNCATE trxstock, refobat, refjenisobat, refsatuan, reflogoobat, refprodusen, refkodemovingobat,
        refkodemovingpo, refdistributorobat, refprincipalobat, refgolonganobat RESTART IDENTITY"

# copy TABLE "COLUMNS" [WHERE]: same column list on both sides.
copy() {
  local table=$1 cols=$2 where=${3:-}
  lab -c "\\copy (SELECT $cols FROM $table $where) TO STDOUT" | app -c "\\copy $table ($cols) FROM STDIN"
  printf '  %-20s %s rows\n' "$table" "$(app -At -c "SELECT count(*) FROM $table")"
}

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisobat       "id, info, sts_hmlm"
copy refsatuan          "id, info"
copy reflogoobat        "id, info"
copy refprodusen        "id, nama, alamat, telp, fax, create_user, create_date, update_user, update_date"
copy refkodemovingobat  "id, kodemoving"
copy refkodemovingpo    "id, kodemovingpo"
copy refdistributorobat "id, distributor, alamat, telp, fax, cp, telp_cp, create_user, create_date, update_user, update_date"
copy refprincipalobat   "id, principal"
copy refgolonganobat    "id, golonganobat"
copy refobat "id, nama, isi, id_jenis, id_logo, id_prod, sat_beli, sat_jual, qjb, hbeli, hjual, hp, non_aktif,
              create_user, create_date, update_user, update_date, id_moving, id_distributor, id_principal,
              id_golongan_obat, komposisi, id_moving_po, kena_pajak, isi_per_box"

branches=$(app -At -c "SELECT string_agg(quote_literal(kode), ',') FROM refbranch")
copy trxstock "id_obat, stok, stok_min, cabang, hbeli, hp, hjual, isactive" \
  "s WHERE EXISTS (SELECT 1 FROM refobat o WHERE o.id = s.id_obat) AND s.cabang IN (${branches:-''})"

echo "Moving identity sequences past the imported ids..."
app <<'SQL'
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['refjenisobat','refsatuan','reflogoobat','refprodusen','refkodemovingobat',
                           'refkodemovingpo','refdistributorobat','refprincipalobat','refgolonganobat','refobat'] LOOP
    EXECUTE format('SELECT setval(pg_get_serial_sequence(%L, ''id''), COALESCE((SELECT max(id) FROM %I), 0) + 1, false)', t, t);
  END LOOP;
END $$;
SQL
echo "Done."
