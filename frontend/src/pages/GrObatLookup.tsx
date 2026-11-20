import { useEffect, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, PenerimaanObat } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { useModal } from "../components/FormKit";

const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const qtyFmt = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** What the user typed on a lookup row before picking it. */
export type GrPick = { noBatch: string; byk: number; tglExp: string; disc: number };
type Draft = { noBatch: string; byk: string; tglExp: string; disc: string };

/**
 * The Penerimaan Barang form's "Data Obat" lookup: the drugs stocked at the
 * chosen branch with its buying price and stock. Each row takes No Batch,
 * Banyak, Exp Date and Disc % (defaults 0, 1, a year from today, 0); the tick
 * adds it to the receipt.
 */
export default function GrObatLookup({
  cabang,
  branchName,
  defaultExp,
  onPick,
  onClose,
}: {
  cabang: string;
  branchName: string;
  /** Exp Date suggested for every row (a year from today). */
  defaultExp: string;
  onPick: (o: PenerimaanObat, p: GrPick) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<PenerimaanObat[] | null>(null);
  const [drafts, setDrafts] = useState<Record<number, Draft>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .penerimaanObat(token, cabang)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load drugs");
      });
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  const draftOf = (id: number): Draft => drafts[id] ?? { noBatch: "0", byk: "1", tglExp: defaultExp, disc: "0" };
  const set = (id: number, patch: Partial<Draft>) => setDrafts((d) => ({ ...d, [id]: { ...draftOf(id), ...patch } }));

  const pick = (o: PenerimaanObat) => {
    const d = draftOf(o.id);
    const byk = Number(d.byk.trim().replace(",", "."));
    const disc = Number((d.disc.trim() || "0").replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0) return setError(`Banyak untuk ${o.nama} harus lebih dari 0.`);
    if (!Number.isFinite(disc) || disc < 0 || disc > 100) return setError(`Disc % untuk ${o.nama} harus antara 0 dan 100.`);
    if (!d.tglExp) return setError(`Isi Exp Date untuk ${o.nama}.`);
    if (d.noBatch.trim().length > 20) return setError(`No Batch untuk ${o.nama} maksimal 20 karakter.`);
    setError("");
    onPick(o, { noBatch: d.noBatch.trim() || "0", byk, tglExp: d.tglExp, disc });
  };

  const columns: Column<PenerimaanObat>[] = [
    { key: "id", header: "ID", value: (o) => String(o.id), sortValue: (o) => o.id },
    { key: "nama", header: "Obat", value: (o) => o.nama, className: "dt-wrap" },
    { key: "satuan", header: "Satuan", value: (o) => o.satuan || "-" },
    { key: "harga", header: "Harga", value: (o) => money.format(o.harga), sortValue: (o) => o.harga, className: "dt-num" },
    { key: "stok", header: "Stok", value: (o) => qtyFmt.format(o.stok), sortValue: (o) => o.stok, className: "dt-num" },
    {
      key: "noBatch",
      header: "No Batch",
      value: () => "",
      sortable: false,
      render: (o) => (
        <input className="gr-input" maxLength={20} aria-label={`No Batch ${o.nama}`} value={draftOf(o.id).noBatch} onChange={(e) => set(o.id, { noBatch: e.target.value })} />
      ),
    },
    {
      key: "byk",
      header: "Banyak",
      value: () => "",
      sortable: false,
      render: (o) => (
        <input
          className="gr-input num"
          type="number"
          min={0}
          step="any"
          aria-label={`Banyak ${o.nama}`}
          value={draftOf(o.id).byk}
          onChange={(e) => set(o.id, { byk: e.target.value })}
          onKeyDown={(e) => e.key === "Enter" && pick(o)}
        />
      ),
    },
    {
      key: "tglExp",
      header: "Exp Date",
      value: () => "",
      sortable: false,
      render: (o) => (
        <input className="gr-input" type="date" aria-label={`Exp Date ${o.nama}`} value={draftOf(o.id).tglExp} onChange={(e) => set(o.id, { tglExp: e.target.value })} />
      ),
    },
    {
      key: "disc",
      header: "Disc %",
      value: () => "",
      sortable: false,
      render: (o) => (
        <input
          className="gr-input num"
          type="number"
          min={0}
          max={100}
          step="any"
          aria-label={`Disc % ${o.nama}`}
          value={draftOf(o.id).disc}
          onChange={(e) => set(o.id, { disc: e.target.value })}
          onKeyDown={(e) => e.key === "Enter" && pick(o)}
        />
      ),
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (o) => (
        <button type="button" className="row-action" aria-label={`Tambah ${o.nama}`} title="Tambah" onClick={() => pick(o)}>
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="gr-obat-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="gr-obat-title">Data Obat · {branchName}</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        {error && (
          <p className="alert error" role="alert">
            {error}
          </p>
        )}
        {!rows ? (
          !error && <p className="muted">Loading…</p>
        ) : (
          <DataTable
            title="Data Obat"
            columns={columns}
            rows={rows}
            rowKey={(o) => o.id}
            initialSort={{ key: "id", dir: "asc" }}
            pageSizes={[5, 10, 20, 50]}
            defaultPageSize={5}
            exports={false}
          />
        )}
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}
