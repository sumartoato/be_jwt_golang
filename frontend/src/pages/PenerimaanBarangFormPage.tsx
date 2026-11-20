import { ReactNode, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Check,
  ChevronDown,
  ChevronUp,
  FilePlus2,
  HeartPulse,
  Info,
  Pencil,
  Plus,
  Printer,
  Save,
  Search,
  Trash2,
  X,
} from "lucide-react";
import {
  api,
  ApiError,
  PenerimaanBarangOptions,
  PenerimaanDetail,
  PenerimaanDistributor,
  PenerimaanObat,
  PenerimaanSaveResult,
} from "../api";
import { useAuth } from "../auth/AuthContext";
import GrDistributorLookup from "./GrDistributorLookup";
import GrObatLookup, { GrPick } from "./GrObatLookup";

const LIST_PATH = "/transaksi-klinik/penerimaan-barang";
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** A drug line of the receipt. */
type Line = {
  key: number;
  idObat: number;
  nama: string;
  jenis: string;
  satuan: string;
  harga: number;
  noBatch: string;
  byk: number;
  tglExp: string;
  disc: number;
};

/** Harga × Banyak less the discount (%). */
const jumlah = (l: Line) => l.harga * l.byk * (1 - l.disc / 100);

/** A year after a YYYY-MM-DD date (the legacy default Exp Date). */
function yearAfter(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCFullYear(d.getUTCFullYear() + 1);
  return d.toISOString().slice(0, 10);
}

/** Card with a coloured header that can be collapsed, like the legacy panels. */
function Card({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(true);
  return (
    <section className="tx-card">
      <header className="tx-card-head">
        <h2>
          <HeartPulse aria-hidden="true" /> {title}
        </h2>
        <button
          type="button"
          className="tx-card-tool"
          aria-expanded={open}
          aria-label={open ? `Collapse ${title}` : `Expand ${title}`}
          onClick={() => setOpen((o) => !o)}
        >
          {open ? (
            <ChevronUp aria-hidden="true" />
          ) : (
            <ChevronDown aria-hidden="true" />
          )}
        </button>
      </header>
      {open && <div className="tx-card-body">{children}</div>}
    </section>
  );
}

/**
 * Transaksi Klinik > Penerimaan Barang > Add (legacy /GoodReceive/New): the
 * goods receipt form. Save records the receipt (No Trans) and adds its drugs to
 * the branch stock; the form is then read-only. New starts an empty form.
 *
 * At .../view/:id it is the View page: a saved receipt, read-only (receipts
 * are not edited), with Kembali and Print.
 */
export default function PenerimaanBarangFormPage() {
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  const { id } = useParams();
  const viewId = id ? Number(id) : null;
  const [options, setOptions] = useState<PenerimaanBarangOptions | null>(null);
  const [detail, setDetail] = useState<PenerimaanDetail | null>(null);
  const [loadError, setLoadError] = useState("");
  const [formKey, setFormKey] = useState(0);

  useEffect(() => {
    if (!token) return;
    setLoadError("");
    setDetail(null);
    Promise.all([
      api.penerimaanBarangOptions(token),
      viewId ? api.penerimaanDetail(token, viewId) : Promise.resolve(null),
    ])
      .then(([o, d]) => {
        setOptions(o);
        setDetail(d);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401)
          logout("Your session has expired. Please log in again.");
        else if (err instanceof ApiError && err.status === 404)
          setLoadError(
            `Penerimaan ${id} tidak ditemukan atau bukan dari cabang Anda.`,
          );
        else
          setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, [viewId]); // eslint-disable-line react-hooks/exhaustive-deps

  const heading = viewId ? "View Penerimaan Barang" : "Add Penerimaan Barang";
  return (
    <section className="panel tx-page">
      {loadError ? (
        <>
          <h1>{heading}</h1>
          <p className="alert error" role="alert">
            {loadError}
          </p>
        </>
      ) : !options || (viewId && !detail) ? (
        <>
          <h1>{heading}</h1>
          <p className="muted">Loading…</p>
        </>
      ) : (
        <GrForm
          key={viewId ? `view-${viewId}` : formKey}
          options={options}
          detail={detail}
          onNew={() =>
            viewId ? navigate(`${LIST_PATH}/new`) : setFormKey((k) => k + 1)
          }
          onCancel={() => navigate(LIST_PATH)}
        />
      )}
    </section>
  );
}

function GrForm({
  options,
  detail,
  onNew,
  onCancel,
}: {
  options: PenerimaanBarangOptions;
  /** A saved receipt (the View page): shown read-only. */
  detail: PenerimaanDetail | null;
  onNew: () => void;
  onCancel: () => void;
}) {
  const { token, logout } = useAuth();
  const isView = detail !== null;
  const [cabang, setCabang] = useState(
    detail?.cabang ??
      (options.branches.length === 1 ? options.branches[0].kode : ""),
  );
  const [distributor, setDistributor] = useState<PenerimaanDistributor | null>(
    detail
      ? {
          id: detail.idDistributor ?? 0,
          nama: detail.distributor || "-",
          alamat: "",
        }
      : null,
  );
  const [noGr, setNoGr] = useState(detail?.noGr ?? "");
  const [tglGr, setTglGr] = useState(detail?.tglGr ?? options.today);
  const [lines, setLines] = useState<Line[]>(
    (detail?.obat ?? []).map((o, i) => ({
      key: i + 1,
      idObat: o.idObat,
      nama: o.nama,
      jenis: o.jenis,
      satuan: o.satuan,
      harga: o.harga,
      noBatch: o.noBatch,
      byk: o.byk,
      tglExp: o.tglExp ?? "",
      disc: o.disc,
    })),
  );
  const [nextKey, setNextKey] = useState((detail?.obat.length ?? 0) + 1);
  const [editing, setEditing] = useState<{
    key: number;
    noBatch: string;
    byk: string;
    tglExp: string;
    disc: string;
  } | null>(null);
  const [distOpen, setDistOpen] = useState(false);
  const [obatOpen, setObatOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState<PenerimaanSaveResult | null>(
    detail ? { id: detail.id, total: detail.total, noClose: null } : null,
  );
  const locked = saved !== null;

  const branchName =
    options.branches.find((b) => b.kode === cabang)?.nama ?? cabang;
  const total = lines.reduce((t, l) => t + jumlah(l), 0);
  const missing = [
    !cabang && "Cabang",
    !distributor && "Distributor",
    !noGr.trim() && "No GR",
    !tglGr && "Tanggal GR",
    lines.length === 0 && "minimal satu obat",
  ].filter(Boolean) as string[];

  // Prices and stock are per branch: another branch empties the drug list.
  const changeCabang = (kode: string) => {
    if (kode !== cabang && lines.length > 0) {
      setLines([]);
      setEditing(null);
      setNotice(
        "Cabang diganti: daftar obat dikosongkan (harga dan stok per cabang).",
      );
    }
    setCabang(kode);
  };
  const openObat = () => {
    if (!cabang) {
      setNotice("Pilih Cabang dulu: daftar obat ditampilkan per cabang.");
      document.getElementById("gr-cabang")?.focus();
      return;
    }
    setNotice("");
    setObatOpen(true);
  };
  const addObat = (o: PenerimaanObat, p: GrPick) => {
    setLines((list) => [
      ...list,
      {
        key: nextKey,
        idObat: o.id,
        nama: o.nama,
        jenis: o.jenis,
        satuan: o.satuan,
        harga: o.harga,
        ...p,
      },
    ]);
    setNextKey((k) => k + 1);
    setObatOpen(false);
  };
  const saveEdit = (l: Line) => {
    if (!editing) return;
    const byk = Number(editing.byk.trim().replace(",", "."));
    const disc = Number((editing.disc.trim() || "0").replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0)
      return setNotice(`Banyak untuk ${l.nama} harus lebih dari 0.`);
    if (!Number.isFinite(disc) || disc < 0 || disc > 100)
      return setNotice(`Disc % untuk ${l.nama} harus antara 0 dan 100.`);
    if (!editing.tglExp) return setNotice(`Isi Exp Date untuk ${l.nama}.`);
    setLines((list) =>
      list.map((x) =>
        x.key === l.key
          ? {
              ...x,
              noBatch: editing.noBatch.trim() || "0",
              byk,
              tglExp: editing.tglExp,
              disc,
            }
          : x,
      ),
    );
    setEditing(null);
    setNotice("");
  };

  async function save() {
    if (!token || locked) return;
    setError("");
    if (missing.length)
      return setError(`Lengkapi dulu: ${missing.join(", ")}.`);
    if (noGr.trim().length > 15) return setError("No GR maksimal 15 karakter.");
    if (editing)
      return setError(
        "Selesaikan dulu ubahan pada daftar obat (centang atau batal).",
      );
    setSaving(true);
    try {
      const res = await api.savePenerimaan(token, {
        cabang,
        idDistributor: distributor!.id,
        noGr: noGr.trim(),
        tglGr,
        obat: lines.map((l) => ({
          idObat: l.idObat,
          noBatch: l.noBatch,
          byk: l.byk,
          tglExp: l.tglExp,
          disc: l.disc,
        })),
      });
      setSaved(res);
      setNotice("");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401)
        return logout("Your session has expired. Please log in again.");
      setError(
        err instanceof ApiError
          ? (Object.values(err.fieldErrors)[0] ?? err.message)
          : "Gagal menyimpan penerimaan",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>
          {isView
            ? `View Penerimaan Barang ${detail.id}`
            : saved
              ? `Penerimaan Barang ${saved.id}`
              : "Add Penerimaan Barang"}
        </h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={onCancel}>
            {saved ? "Kembali" : "Cancel"}
          </button>
          {!isView && (
            <button
              type="button"
              className="btn-small"
              title="Form baru"
              onClick={onNew}
            >
              New <FilePlus2 aria-hidden="true" />
            </button>
          )}
          <button
            type="button"
            className="btn-add"
            disabled
            title="Cetak penerimaan barang belum tersedia"
          >
            Print <Printer aria-hidden="true" />
          </button>
          {!isView && (
            <button
              type="button"
              className="btn-small btn-primary"
              onClick={save}
              disabled={saving || locked}
              title={
                locked
                  ? "Sudah disimpan"
                  : missing.length
                    ? `Lengkapi: ${missing.join(", ")}`
                    : "Simpan penerimaan"
              }
            >
              {saving ? "Saving…" : locked ? "Saved" : "Save"}{" "}
              <Save aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
      {isView && (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Penerimaan tersimpan, hanya untuk dilihat
          {detail.tglTrans ? ` · dibuat ${detail.tglTrans}` : ""}
          {detail.createUser ? ` oleh ${detail.createUser}` : ""}
          {detail.supplier ? ` · Supplier ${detail.supplier}` : ""} · Total{" "}
          {plain.format(detail.total)}.
        </p>
      )}
      {saved && !isView && (
        <p className="alert success" role="status">
          Penerimaan tersimpan: No Trans <strong>{saved.id}</strong> · Total{" "}
          {plain.format(saved.total)}. Stok {branchName} sudah ditambah. Klik
          New untuk penerimaan berikutnya.
        </p>
      )}
      {error && (
        <p className="alert error" role="alert">
          {error}
          <button
            type="button"
            className="row-action alert-close"
            aria-label="Dismiss"
            onClick={() => setError("")}
          >
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      {notice && (
        <p className="alert info" role="status">
          {notice}
          <button
            type="button"
            className="row-action alert-close"
            aria-label="Dismiss"
            onClick={() => setNotice("")}
          >
            <X aria-hidden="true" />
          </button>
        </p>
      )}

      <fieldset className="tx-fieldset tx-head gr-head" disabled={locked}>
        <div>
          <div className="tx-row">
            <label htmlFor="gr-notrans">No Trans</label>
            <input
              id="gr-notrans"
              readOnly
              placeholder="Otomatis saat disimpan"
              value={saved ? String(saved.id) : ""}
            />
          </div>
          <div className="tx-row">
            <label htmlFor="gr-cabang">
              Cabang<span className="req"> *</span>
            </label>
            <select
              id="gr-cabang"
              className="select"
              value={cabang}
              onChange={(e) => changeCabang(e.target.value)}
            >
              <option value="">-- Pilih Cabang --</option>
              {options.branches.map((b) => (
                <option key={b.kode} value={b.kode}>
                  {b.nama || b.kode}
                </option>
              ))}
            </select>
          </div>
          <div className="tx-row">
            <label htmlFor="gr-distributor">
              Distributor<span className="req"> *</span>
            </label>
            <div className="tx-lookup">
              <input
                id="gr-distributor"
                readOnly
                placeholder="Distributor"
                value={distributor ? distributor.nama : ""}
              />
              <button
                type="button"
                className="tx-lookup-btn"
                aria-label="Cari Distributor"
                title="Cari"
                onClick={() => setDistOpen(true)}
              >
                <Search aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>
        <div>
          <div className="tx-row">
            <label htmlFor="gr-nogr">
              No GR<span className="req"> *</span>
            </label>
            <input
              id="gr-nogr"
              maxLength={15}
              placeholder="No GR Max 15 Char"
              value={noGr}
              onChange={(e) => setNoGr(e.target.value)}
            />
          </div>
          <div className="tx-row">
            <label htmlFor="gr-tgl">Tanggal GR</label>
            <input
              id="gr-tgl"
              type="date"
              max={options.today}
              value={tglGr}
              onChange={(e) => setTglGr(e.target.value || options.today)}
            />
          </div>
        </div>
      </fieldset>

      <Card title="Daftar Obat">
        <fieldset className="tx-fieldset" disabled={locked}>
          <div className="tx-items-head">
            <p className="tx-sum" aria-live="polite">
              Total : {plain.format(total)} · {lines.length} obat
            </p>
            {!isView && (
              <button type="button" className="btn-add" onClick={openObat}>
                <Plus aria-hidden="true" /> Add Obat
              </button>
            )}
          </div>
          <div className="table-wrap">
            <table className="data-table tx-items">
              <thead>
                <tr>
                  {[
                    "ID",
                    "Obat",
                    "Jenis",
                    "Satuan",
                    "Harga",
                    "No Batch",
                    "Banyak",
                    "Jumlah",
                    "Exp Date",
                    "Disc %",
                    ...(isView ? [] : ["Action"]),
                  ].map((c) => (
                    <th
                      key={c}
                      className={
                        ["Harga", "Banyak", "Jumlah", "Disc %"].includes(c)
                          ? "dt-num"
                          : undefined
                      }
                    >
                      {c}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {lines.length === 0 ? (
                  <tr>
                    <td colSpan={isView ? 10 : 11} className="dt-empty">
                      Belum ada obat.
                    </td>
                  </tr>
                ) : (
                  lines.map((l) => {
                    const edit = editing?.key === l.key ? editing : null;
                    return (
                      <tr key={l.key}>
                        <td>{l.idObat}</td>
                        <td>{l.nama}</td>
                        <td>{l.jenis || "-"}</td>
                        <td>{l.satuan || "-"}</td>
                        <td className="dt-num">{plain.format(l.harga)}</td>
                        <td>
                          {edit ? (
                            <input
                              className="gr-input"
                              maxLength={20}
                              aria-label={`No Batch ${l.nama}`}
                              value={edit.noBatch}
                              onChange={(e) =>
                                setEditing({ ...edit, noBatch: e.target.value })
                              }
                            />
                          ) : (
                            l.noBatch
                          )}
                        </td>
                        <td className="dt-num">
                          {edit ? (
                            <input
                              className="gr-input num"
                              type="number"
                              min={0}
                              step="any"
                              aria-label={`Banyak ${l.nama}`}
                              value={edit.byk}
                              onChange={(e) =>
                                setEditing({ ...edit, byk: e.target.value })
                              }
                              onKeyDown={(e) =>
                                e.key === "Enter" && saveEdit(l)
                              }
                            />
                          ) : (
                            plain.format(l.byk)
                          )}
                        </td>
                        <td className="dt-num">{plain.format(jumlah(l))}</td>
                        <td>
                          {edit ? (
                            <input
                              className="gr-input"
                              type="date"
                              aria-label={`Exp Date ${l.nama}`}
                              value={edit.tglExp}
                              onChange={(e) =>
                                setEditing({ ...edit, tglExp: e.target.value })
                              }
                            />
                          ) : (
                            l.tglExp.split("-").reverse().join("/")
                          )}
                        </td>
                        <td className="dt-num">
                          {edit ? (
                            <input
                              className="gr-input num"
                              type="number"
                              min={0}
                              max={100}
                              step="any"
                              aria-label={`Disc % ${l.nama}`}
                              value={edit.disc}
                              onChange={(e) =>
                                setEditing({ ...edit, disc: e.target.value })
                              }
                              onKeyDown={(e) =>
                                e.key === "Enter" && saveEdit(l)
                              }
                            />
                          ) : (
                            plain.format(l.disc)
                          )}
                        </td>
                        {!isView && (
                          <td className="tx-actions">
                            {edit ? (
                              <>
                                <button
                                  type="button"
                                  className="row-action"
                                  aria-label={`Simpan ${l.nama}`}
                                  title="Simpan"
                                  onClick={() => saveEdit(l)}
                                >
                                  <Check aria-hidden="true" />
                                </button>
                                <button
                                  type="button"
                                  className="row-action"
                                  aria-label={`Batal ubah ${l.nama}`}
                                  title="Batal"
                                  onClick={() => setEditing(null)}
                                >
                                  <X aria-hidden="true" />
                                </button>
                              </>
                            ) : (
                              <>
                                <button
                                  type="button"
                                  className="row-action danger"
                                  aria-label={`Hapus ${l.nama}`}
                                  title="Hapus"
                                  onClick={() =>
                                    setLines((list) =>
                                      list.filter((x) => x.key !== l.key),
                                    )
                                  }
                                >
                                  <Trash2 aria-hidden="true" />
                                </button>
                                <button
                                  type="button"
                                  className="row-action"
                                  aria-label={`Ubah ${l.nama}`}
                                  title="Ubah No Batch / Banyak / Exp Date / Disc"
                                  onClick={() =>
                                    setEditing({
                                      key: l.key,
                                      noBatch: l.noBatch,
                                      byk: String(l.byk),
                                      tglExp: l.tglExp,
                                      disc: String(l.disc),
                                    })
                                  }
                                >
                                  <Pencil aria-hidden="true" />
                                </button>
                              </>
                            )}
                          </td>
                        )}
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </fieldset>
      </Card>

      {distOpen && (
        <GrDistributorLookup
          onPick={(d) => {
            setDistributor(d);
            setDistOpen(false);
          }}
          onClose={() => setDistOpen(false)}
        />
      )}
      {obatOpen && (
        <GrObatLookup
          cabang={cabang}
          branchName={branchName}
          defaultExp={yearAfter(options.today)}
          onPick={addObat}
          onClose={() => setObatOpen(false)}
        />
      )}
    </>
  );
}
