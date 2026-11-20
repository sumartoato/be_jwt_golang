import { FormEvent, useEffect, useState } from "react";
import { Download, FileText, HeartPulse } from "lucide-react";
import { api, ApiError, ReportPendapatanOptions } from "../api";
import { useAuth } from "../auth/AuthContext";
import { downloadPendapatan } from "./pendapatanPdf";

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > Report Pendapatan (legacy /ReportPendapatan): the "Data
 * Pendapatan" form. Cabang is required; Dokter lists the chosen branch's
 * doctors. Download builds the "Laporan Pendapatan Klinik" PDF
 * (Report_Income.pdf); Clear resets the form.
 */
export default function ReportPendapatanPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<ReportPendapatanOptions | null>(null);
  const [cabang, setCabang] = useState("");
  const [layanan, setLayanan] = useState("");
  const [dokter, setDokter] = useState("");
  const [dokterList, setDokterList] = useState<{ id: number; name: string }[]>([]);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");
  const fail = (err: unknown, what: string) => {
    if (err instanceof ApiError && err.status === 401) return expired();
    setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : what);
  };

  const reset = (o: ReportPendapatanOptions) => {
    setCabang(o.branches.length === 1 ? o.branches[0].kode : "");
    setLayanan("");
    setDokter("");
    setFrom(dayBefore(o.today));
    setTo(o.today);
    setError("");
    setDone("");
  };

  useEffect(() => {
    if (!token) return;
    api
      .reportPendapatanOptions(token)
      .then((o) => {
        setOptions(o);
        reset(o);
      })
      .catch((err) => fail(err, "Failed to load"));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Dokter follows the chosen branch: its doctors, and "All" again on a change.
  useEffect(() => {
    setDokter("");
    setDokterList([]);
    if (!token || !cabang) return;
    let live = true;
    api
      .reportPendapatanDokter(token, cabang)
      .then((res) => live && setDokterList(res.rows))
      .catch((err) => live && fail(err, "Gagal memuat dokter"));
    return () => {
      live = false;
    };
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  async function download(e: FormEvent) {
    e.preventDefault();
    if (!token || !options) return;
    setError("");
    setDone("");
    if (!cabang) return setError("Pilih Cabang dulu.");
    if (!from || !to || to < from) return setError("Isi Dari Tanggal dan Sampai Tanggal (Dari ≤ Sampai).");
    setBusy(true);
    try {
      const res = await api.reportPendapatan(token, {
        cabang,
        from,
        to,
        pelayanan: layanan ? Number(layanan) : undefined,
        dokter: dokter ? Number(dokter) : undefined,
      });
      const layananName = options.pelayanan.find((o) => String(o.id) === layanan)?.name ?? "All";
      const dokterName = dokterList.find((o) => String(o.id) === dokter)?.name ?? "All";
      await downloadPendapatan(res, layananName, dokterName);
      setDone(`Report_Income.pdf diunduh · ${res.rows.length} transaksi.`);
    } catch (err) {
      fail(err, "Gagal membuat laporan");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Report Pendapatan</h1>
      </div>
      {!options ? (
        error ? (
          <p className="alert error" role="alert">
            {error}
          </p>
        ) : (
          <p className="muted">Loading…</p>
        )
      ) : (
        <form className="tx-card income-card" onSubmit={download} noValidate>
          <header className="tx-card-head">
            <h2>
              <HeartPulse aria-hidden="true" /> Data Pendapatan
            </h2>
          </header>
          <div className="tx-card-body income-grid">
            <div className="tx-row">
              <label htmlFor="inc-cabang">Cabang</label>
              <select id="inc-cabang" className="select" value={cabang} onChange={(e) => setCabang(e.target.value)}>
                <option value="">-- Pilih Cabang --</option>
                {options.branches.map((b) => (
                  <option key={b.kode} value={b.kode}>
                    {b.nama || b.kode}
                  </option>
                ))}
              </select>
            </div>
            <div className="tx-row">
              <label htmlFor="inc-layanan">Layanan</label>
              <select id="inc-layanan" className="select" value={layanan} onChange={(e) => setLayanan(e.target.value)}>
                <option value="">All</option>
                {options.pelayanan.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="tx-row">
              <label htmlFor="inc-dokter">Dokter</label>
              <select id="inc-dokter" className="select" value={dokter} onChange={(e) => setDokter(e.target.value)} disabled={!cabang}>
                <option value="">All</option>
                {dokterList.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="tx-row">
              <label htmlFor="inc-from">Dari Tanggal</label>
              <input id="inc-from" type="date" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div className="tx-row">
              <label htmlFor="inc-to">Sampai Tanggal</label>
              <input id="inc-to" type="date" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
            </div>
          </div>
          {(error || done) && (
            <div className="income-msg">
              {error && (
                <p className="alert error" role="alert">
                  {error}
                </p>
              )}
              {done && (
                <p className="alert success" role="status">
                  {done}
                </p>
              )}
            </div>
          )}
          <footer className="income-foot">
            <button type="button" className="btn-small" onClick={() => reset(options)}>
              Clear <FileText aria-hidden="true" />
            </button>
            <button type="submit" className="btn-add" disabled={busy}>
              {busy ? "Membuat…" : "Download"} <Download aria-hidden="true" />
            </button>
          </footer>
        </form>
      )}
    </section>
  );
}
