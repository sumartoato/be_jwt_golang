import { ReactNode, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Check, ChevronDown, ChevronUp, FilePlus2, HeartPulse, Info, Pencil, Plus, Printer, Receipt, Save, Search, Trash2, X } from "lucide-react";
import { api, ApiError, PengobatanObat, PengobatanOptions, PengobatanPendaftaran } from "../api";
import { useAuth } from "../auth/AuthContext";
import { age, fmtDate } from "../components/dates";
import DokterLookup from "./DokterLookup";
import ObatLookup from "./ObatLookup";
import { jumlah, ObatLine, obatTotals, ppn } from "./pengobatanCalc";
import PendaftaranLookup from "./PendaftaranLookup";

/**
 * Transaksi Klinik > Pengobatan > Add: the treatment form, laid out like the
 * legacy PengobatanDetail/New screen. This is a mockup: the dropdowns use real
 * options, but lookups, item lists and saving are not wired yet.
 */

const LIST_PATH = "/transaksi-klinik/pengobatan";
const money = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

type Tab = "obat" | "tindakan" | "penunjang" | "diagnosa" | "pemeriksaan";
const TABS: { key: Tab; label: string }[] = [
  { key: "obat", label: "Obat" },
  { key: "tindakan", label: "Tindakan" },
  { key: "penunjang", label: "Penunjang" },
  { key: "diagnosa", label: "Diagnosa" },
  { key: "pemeriksaan", label: "Pemeriksaan" },
];

/** Card with a coloured header that can be collapsed, like the legacy panels. */
function Card({ title, children, className }: { title: string; children: ReactNode; className?: string }) {
  const [open, setOpen] = useState(true);
  return (
    <section className={["tx-card", className].filter(Boolean).join(" ")}>
      <header className="tx-card-head">
        <h2>
          <HeartPulse aria-hidden="true" /> {title}
        </h2>
        <button type="button" className="tx-card-tool" aria-expanded={open} aria-label={open ? `Collapse ${title}` : `Expand ${title}`} onClick={() => setOpen((o) => !o)}>
          {open ? <ChevronUp aria-hidden="true" /> : <ChevronDown aria-hidden="true" />}
        </button>
      </header>
      {open && <div className="tx-card-body">{children}</div>}
    </section>
  );
}

/** Label on the left, control(s) on the right. */
function Row({ id, label, required, children }: { id?: string; label: string; required?: boolean; children: ReactNode }) {
  return (
    <div className="tx-row">
      <label htmlFor={id}>
        {label}
        {required && <span className="req"> *</span>}
      </label>
      <div className="tx-ctrl">{children}</div>
    </div>
  );
}

/** Read-only input with a search button that opens a lookup. */
function Lookup({ id, placeholder, value = "", onSearch }: { id: string; placeholder: string; value?: string; onSearch: () => void }) {
  return (
    <div className="tx-lookup">
      <input id={id} readOnly placeholder={placeholder} value={value} />
      <button type="button" className="tx-lookup-btn" aria-label={`Cari ${placeholder}`} title="Cari" onClick={onSearch}>
        <Search aria-hidden="true" />
      </button>
    </div>
  );
}

/** Empty item table with the given columns. */
function ItemTable({ columns, empty }: { columns: string[]; empty: string }) {
  return (
    <div className="table-wrap">
      <table className="data-table tx-items">
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c}>{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          <tr>
            <td colSpan={columns.length} className="dt-empty">
              {empty}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

const num = (s: string) => (Number.isFinite(Number(s)) ? Number(s) : 0);
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const money2 = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** The Obat tab's list: Banyak and Dosis can be edited in place, a line can be removed. */
function ObatTable({
  lines,
  onChange,
  onRemove,
  onError,
}: {
  lines: ObatLine[];
  onChange: (idObat: number, byk: number, dosis: string) => void;
  onRemove: (idObat: number) => void;
  onError: (msg: string) => void;
}) {
  const [editing, setEditing] = useState<{ idObat: number; byk: string; dosis: string } | null>(null);
  const save = (l: ObatLine) => {
    if (!editing) return;
    const byk = Number(editing.byk.trim().replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0) return onError(`Banyak untuk ${l.nama} harus lebih dari 0.`);
    if (byk > l.stok) return onError(`Stok ${l.nama} tidak cukup: stok ${plain.format(l.stok)}, diminta ${plain.format(byk)}.`);
    onChange(l.idObat, byk, editing.dosis.trim());
    setEditing(null);
  };
  const cols = ["ID", "Obat", "Jenis", "Satuan", "Harga", "Banyak", "Jumlah", "PPN", "Dosis", "Action"];
  return (
    <div className="table-wrap">
      <table className="data-table tx-items">
        <thead>
          <tr>
            {cols.map((c) => (
              <th key={c} className={["Harga", "Banyak", "Jumlah", "PPN"].includes(c) ? "dt-num" : undefined}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {lines.length === 0 ? (
            <tr>
              <td colSpan={cols.length} className="dt-empty">
                Belum ada obat.
              </td>
            </tr>
          ) : (
            lines.map((l) => {
              const edit = editing?.idObat === l.idObat ? editing : null;
              return (
                <tr key={l.idObat}>
                  <td>{l.idObat}</td>
                  <td>{l.nama}</td>
                  <td>{l.jenis || "-"}</td>
                  <td>{l.satuan || "-"}</td>
                  <td className="dt-num">{plain.format(l.harga)}</td>
                  <td className="dt-num">
                    {edit ? (
                      <input
                        className="qty-input"
                        type="number"
                        min={0}
                        step="any"
                        aria-label={`Banyak ${l.nama}`}
                        value={edit.byk}
                        onChange={(e) => setEditing({ ...edit, byk: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      plain.format(l.byk)
                    )}
                  </td>
                  <td className="dt-num">{plain.format(jumlah(l))}</td>
                  <td className="dt-num">{plain.format(ppn(l))}</td>
                  <td>
                    {edit ? (
                      <input
                        className="dosis-input"
                        aria-label={`Dosis ${l.nama}`}
                        placeholder="3 x 1"
                        maxLength={100}
                        value={edit.dosis}
                        onChange={(e) => setEditing({ ...edit, dosis: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      l.dosis
                    )}
                  </td>
                  <td className="tx-actions">
                    {edit ? (
                      <>
                        <button type="button" className="row-action" aria-label={`Simpan ${l.nama}`} title="Simpan" onClick={() => save(l)}>
                          <Check aria-hidden="true" />
                        </button>
                        <button type="button" className="row-action" aria-label={`Batal ubah ${l.nama}`} title="Batal" onClick={() => setEditing(null)}>
                          <X aria-hidden="true" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button type="button" className="row-action danger" aria-label={`Hapus ${l.nama}`} title="Hapus" onClick={() => onRemove(l.idObat)}>
                          <Trash2 aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className="row-action"
                          aria-label={`Ubah ${l.nama}`}
                          title="Ubah banyak / dosis"
                          onClick={() => setEditing({ idObat: l.idObat, byk: String(l.byk), dosis: l.dosis })}
                        >
                          <Pencil aria-hidden="true" />
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}

export default function PengobatanFormPage() {
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  const [options, setOptions] = useState<PengobatanOptions | null>(null);
  const [loadError, setLoadError] = useState("");
  const [formKey, setFormKey] = useState(0);

  useEffect(() => {
    if (!token) return;
    api
      .pengobatanOptions(token)
      .then(setOptions)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <section className="panel tx-page">
      <div className="page-head">
        <h1>Add Pengobatan</h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={() => navigate(LIST_PATH)}>
            Cancel
          </button>
          <button type="button" className="btn-small" title="Kosongkan form" onClick={() => setFormKey((k) => k + 1)}>
            New <FilePlus2 aria-hidden="true" />
          </button>
          <button type="button" className="btn-add" disabled title="Tersedia setelah disimpan">
            Kuitansi <Receipt aria-hidden="true" />
          </button>
          <button type="button" className="btn-add" disabled title="Tersedia setelah disimpan">
            Print <Printer aria-hidden="true" /> <ChevronDown aria-hidden="true" />
          </button>
          <button type="button" className="btn-small btn-primary" disabled title="Belum tersedia: form ini masih mockup">
            Save <Save aria-hidden="true" />
          </button>
        </div>
      </div>
      <p className="alert info tx-mock-note" role="note">
        <Info aria-hidden="true" /> Mockup: No. Daftar, Dokter PJ dan Add Obat sudah tersambung. Pencarian pasien dan jamkes, Tindakan, Penunjang, Diagnosa dan Save belum.
      </p>
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !options ? (
        <p className="muted">Loading…</p>
      ) : (
        <PengobatanForm key={formKey} options={options} />
      )}
    </section>
  );
}

function PengobatanForm({ options }: { options: PengobatanOptions }) {
  const [tab, setTab] = useState<Tab>("obat");
  const [notice, setNotice] = useState("");
  const [payment, setPayment] = useState("");
  const [diskon, setDiskon] = useState("");
  const [tukarPoint, setTukarPoint] = useState("");
  const defaultPelayanan = String(options.pelayanan.find((o) => o.name === "POLI UMUM")?.id ?? options.pelayanan[0]?.id ?? "");
  const defaultBayar = options.pembayaran.find((o) => o.name === "TUNAI")?.id ?? options.pembayaran[0]?.id ?? "";
  const [cabang, setCabang] = useState(options.branches.length === 1 ? options.branches[0].kode : "");
  const [pelayanan, setPelayanan] = useState(defaultPelayanan);
  const [reg, setReg] = useState<PengobatanPendaftaran | null>(null);
  const [dokter, setDokter] = useState<{ id: number; nama: string } | null>(null);
  const [lookupOpen, setLookupOpen] = useState(false);
  const [dokterOpen, setDokterOpen] = useState(false);
  const [obatOpen, setObatOpen] = useState(false);
  const [obat, setObat] = useState<ObatLine[]>([]);
  const notYet = (what: string) => setNotice(`Pencarian ${what} belum tersedia di mockup ini.`);

  // Both lookups list the data of one branch.
  const needBranch = (what: string) => {
    if (cabang) return true;
    setNotice(`Pilih Cabang dulu: data ${what} ditampilkan per cabang.`);
    document.getElementById("tx-cabang")?.focus();
    return false;
  };
  const openLookup = () => {
    if (!needBranch("pendaftaran")) return;
    setNotice("");
    setLookupOpen(true);
  };
  const openDokter = () => {
    if (!needBranch("dokter")) return;
    setNotice("");
    setDokterOpen(true);
  };
  const openObat = () => {
    if (!needBranch("obat")) return;
    setNotice("");
    setObatOpen(true);
  };
  // Picking a drug already in the list adds to its quantity.
  const addObat = (o: PengobatanObat, byk: number) => {
    setObat((list) =>
      list.some((l) => l.idObat === o.id)
        ? list.map((l) => (l.idObat === o.id ? { ...l, byk: l.byk + byk } : l))
        : [...list, { idObat: o.id, nama: o.nama, jenis: o.jenis, satuan: o.satuan, harga: o.harga, byk, kenaPajak: o.kenaPajak, stok: o.stok, dosis: "" }],
    );
    setObatOpen(false);
  };
  const pick = (r: PengobatanPendaftaran) => {
    setReg(r);
    if (r.idJenisRawat !== null) setPelayanan(String(r.idJenisRawat));
    setDokter(r.idDokter !== null ? { id: r.idDokter, nama: r.dokter } : null);
    setLookupOpen(false);
    setNotice("");
  };
  const changeCabang = (kode: string) => {
    setCabang(kode);
    // A registration and its doctor belong to one branch: picking another branch clears them.
    if (reg && reg.cabang !== kode) {
      setReg(null);
      setDokter(null);
      setPelayanan(defaultPelayanan);
      setNotice("Cabang diganti: data pendaftaran, pasien dan Dokter PJ dikosongkan.");
    } else if (dokter && kode !== cabang) {
      setDokter(null);
      setNotice("Cabang diganti: Dokter PJ dikosongkan.");
    }
    // Drug prices and stock are per branch.
    if (obat.length > 0 && kode !== cabang) {
      setObat([]);
      setNotice((n) => (n ? `${n} Daftar obat juga dikosongkan (harga dan stok per cabang).` : "Cabang diganti: daftar obat dikosongkan (harga dan stok per cabang)."));
    }
  };
  const umur = reg ? age(reg.tglLahir) : null;

  // Drugs only for now; procedures and examinations will add to the bill.
  const totals = obatTotals(obat);
  const total = totals.bill;
  const grandTotal = Math.max(total - num(diskon) - num(tukarPoint), 0);
  const sisa = Math.max(grandTotal - num(payment), 0);
  const kembalian = Math.max(num(payment) - grandTotal, 0);

  return (
    <>
      {notice && (
        <p className="alert info" role="status">
          {notice}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice("")}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}

      <div className="tx-head">
        <div className="tx-row">
          <label htmlFor="tx-cabang">
            Cabang<span className="req"> *</span>
          </label>
          <select id="tx-cabang" className="select" value={cabang} onChange={(e) => changeCabang(e.target.value)}>
            <option value="">-- Pilih Cabang --</option>
            {options.branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.nama || b.kode}
              </option>
            ))}
          </select>
        </div>
        <div className="tx-row">
          <label htmlFor="tx-nokwi">No. Kuitansi</label>
          <input id="tx-nokwi" readOnly placeholder="Otomatis saat disimpan" />
        </div>
        <div className="tx-row">
          <label htmlFor="tx-tglkwi">Tgl Kuitansi</label>
          <input id="tx-tglkwi" type="date" defaultValue={options.today} max={options.today} />
        </div>
      </div>

      <div className="tx-grid tx-grid-2">
        <Card title="Data Pendaftaran">
          <Row id="tx-nodaftar" label="No. Daftar" required>
            <Lookup id="tx-nodaftar" placeholder="No Pendaftaran" value={reg ? String(reg.id) : ""} onSearch={openLookup} />
          </Row>
          <Row id="tx-tgldaftar" label="Tanggal Daftar">
            <input id="tx-tgldaftar" readOnly value={reg?.tglMasuk ? `${fmtDate(reg.tglMasuk.slice(0, 10))} ${reg.tglMasuk.slice(11)}` : ""} />
          </Row>
          <Row id="tx-stskun" label="Sts. Kun">
            <input id="tx-stskun" readOnly value={reg?.status ?? ""} />
          </Row>
          <Row id="tx-pelayanan" label="Pelayanan" required>
            <select id="tx-pelayanan" className="select" value={pelayanan} onChange={(e) => setPelayanan(e.target.value)}>
              {options.pelayanan.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </Row>
          <Row id="tx-bayar" label="Jenis Bayar" required>
            <select id="tx-bayar" className="select" defaultValue={defaultBayar}>
              {options.pembayaran.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </Row>
          <Row id="tx-dokter" label="Dokter PJ" required>
            <Lookup id="tx-dokter" placeholder={reg && !dokter ? "Belum di Pilih" : "Dokter"} value={dokter?.nama ?? ""} onSearch={openDokter} />
          </Row>
        </Card>

        <Card title="Data Pasien">
          <Row id="tx-norm" label="No. RM">
            <Lookup id="tx-norm" placeholder="No RM" value={reg?.noRm ?? ""} onSearch={() => notYet("pasien")} />
          </Row>
          <Row id="tx-nama" label="Nama & Umur">
            <div className="tx-split tx-split-name">
              <input id="tx-nama" readOnly value={reg?.nama ?? ""} />
              <input aria-label="Umur" readOnly value={umur === null ? "" : `${umur} Th`} />
            </div>
          </Row>
          <Row id="tx-alamat" label="Alamat">
            <input id="tx-alamat" readOnly value={reg?.alamat ?? ""} />
          </Row>
          <Row id="tx-hp" label="Telp / HP">
            <input id="tx-hp" readOnly value={reg?.hp ?? ""} />
          </Row>
          <Row id="tx-pj" label="Penanggung Jawab">
            <input id="tx-pj" readOnly value={reg?.penanggung ?? ""} />
          </Row>
          <Row id="tx-jamkes" label="Jam Kes / No Kartu">
            <div className="tx-split">
              <Lookup id="tx-jamkes" placeholder="Jamkes" value={reg?.jamkes ?? ""} onSearch={() => notYet("jaminan kesehatan")} />
              <Lookup id="tx-nokartu" placeholder="No Kartu" value={reg?.noJamkes ?? ""} onSearch={() => notYet("kartu jaminan")} />
            </div>
          </Row>
        </Card>
      </div>

      <div className="tx-grid tx-grid-main">
        <Card title="Daftar Pengobatan & Pemeriksaan">
          <div className="tabs" role="tablist" aria-label="Daftar pengobatan">
            {TABS.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                id={`tx-tab-${t.key}`}
                aria-selected={tab === t.key}
                aria-controls={`tx-panel-${t.key}`}
                className={tab === t.key ? "tab active" : "tab"}
                onClick={() => setTab(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>
          <div role="tabpanel" id={`tx-panel-${tab}`} aria-labelledby={`tx-tab-${tab}`} className="tx-tabpanel">
            {tab === "obat" && (
              <>
                <div className="tx-items-head">
                  <div>
                    <button type="button" className="btn-add" onClick={openObat}>
                      <Plus aria-hidden="true" /> Add Obat
                    </button>
                    <p className="tx-sum" aria-live="polite">
                      Total Kena Diskon : {plain.format(totals.kenaDiskon)} di bulatkan : {plain.format(totals.kenaDiskonRounded)}
                    </p>
                  </div>
                  <div className="tx-sum tx-sum-right" aria-live="polite">
                    <p>
                      Total : {plain.format(totals.total)} di bulatkan : {plain.format(totals.totalRounded)}
                    </p>
                    <p>
                      PPN : {plain.format(totals.ppn)} di bulatkan : {plain.format(totals.ppnRounded)}
                    </p>
                    <p>
                      PPN Kena Diskon : {plain.format(totals.ppnKenaDiskon)} di bulatkan : {plain.format(totals.ppnKenaDiskonRounded)}
                    </p>
                  </div>
                </div>
                <ObatTable
                  lines={obat}
                  onChange={(id, byk, dosis) => setObat((list) => list.map((l) => (l.idObat === id ? { ...l, byk, dosis } : l)))}
                  onRemove={(id) => setObat((list) => list.filter((l) => l.idObat !== id))}
                  onError={setNotice}
                />
                <div className="tx-otc">
                  <label htmlFor="tx-otc">Perujuk Obat Bebas / OTC</label>
                  <Lookup id="tx-otc" placeholder="Perujuk OTC" onSearch={() => notYet("perujuk OTC")} />
                </div>
              </>
            )}
            {tab === "tindakan" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => notYet("tindakan")}>
                    <Plus aria-hidden="true" /> Add Tindakan
                  </button>
                  <p className="tx-sum tx-sum-right">Total : 0</p>
                </div>
                <ItemTable columns={["ID", "Tindakan", "Dokter", "Harga", "Banyak", "Jumlah", "Action"]} empty="Belum ada tindakan." />
              </>
            )}
            {tab === "penunjang" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => notYet("penunjang")}>
                    <Plus aria-hidden="true" /> Add Penunjang
                  </button>
                  <p className="tx-sum tx-sum-right">Total : 0</p>
                </div>
                <ItemTable columns={["ID", "Penunjang", "Jenis", "Harga", "Banyak", "Jumlah", "Action"]} empty="Belum ada pemeriksaan penunjang." />
              </>
            )}
            {tab === "diagnosa" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => notYet("diagnosa (ICD-X)")}>
                    <Plus aria-hidden="true" /> Add Diagnosa
                  </button>
                </div>
                <ItemTable columns={["Kode ICD-X", "Nama Penyakit", "Jenis ICD-X", "Keterangan", "Action"]} empty="Belum ada diagnosa." />
              </>
            )}
            {tab === "pemeriksaan" && (
              <div className="tx-exam">
                <Row id="tx-keluhan" label="Keluhan Utama">
                  <textarea id="tx-keluhan" rows={2} />
                </Row>
                <Row id="tx-anamnesis" label="Anamnesis">
                  <textarea id="tx-anamnesis" rows={2} />
                </Row>
                <Row id="tx-sistole" label="Tekanan Darah">
                  <div className="tx-vitals">
                    <input id="tx-sistole" type="number" min={0} aria-label="Sistole" placeholder="Sistole" />
                    <span>/</span>
                    <input type="number" min={0} aria-label="Diastole" placeholder="Diastole" />
                    <span className="unit">mmHg</span>
                  </div>
                </Row>
                <Row id="tx-bb" label="Berat / Tinggi">
                  <div className="tx-vitals">
                    <input id="tx-bb" type="number" min={0} step="any" aria-label="Berat badan" placeholder="Berat" />
                    <span className="unit">kg</span>
                    <input type="number" min={0} step="any" aria-label="Tinggi badan" placeholder="Tinggi" />
                    <span className="unit">cm</span>
                  </div>
                </Row>
                <Row id="tx-suhu" label="Suhu Badan">
                  <div className="tx-vitals">
                    <input id="tx-suhu" type="number" min={0} step="0.1" placeholder="Suhu" />
                    <span className="unit">°C</span>
                  </div>
                </Row>
                <Row id="tx-fisik" label="Pemeriksaan Fisik">
                  <textarea id="tx-fisik" rows={2} />
                </Row>
                <Row id="tx-rekap" label="Rekap Hasil Penunjang">
                  <textarea id="tx-rekap" rows={2} />
                </Row>
                <Row id="tx-hasil" label="Hasil Penunjang">
                  <textarea id="tx-hasil" rows={2} />
                </Row>
                <Row id="tx-rencana" label="Rencana Penatalaksanaan">
                  <textarea id="tx-rencana" rows={2} />
                </Row>
                <Row id="tx-tindakan-dokter" label="Tindakan Dokter">
                  <textarea id="tx-tindakan-dokter" rows={2} />
                </Row>
              </div>
            )}
          </div>
        </Card>

        <Card title="Data Pembayaran" className="tx-pay">
          <output className="tx-total" aria-label="Total tagihan">
            {money2.format(total)}
          </output>
          <Row id="tx-payment" label="Payment">
            <input id="tx-payment" type="number" min={0} step="any" inputMode="decimal" className="num" placeholder="0" value={payment} onChange={(e) => setPayment(e.target.value)} />
          </Row>
          <Row id="tx-diskon" label="Diskon">
            <input id="tx-diskon" type="number" min={0} step="any" inputMode="decimal" className="num" placeholder="0" value={diskon} onChange={(e) => setDiskon(e.target.value)} />
          </Row>
          <Row id="tx-point" label="Tukar Point">
            <input id="tx-point" type="number" min={0} step="any" inputMode="decimal" className="num" placeholder="0" value={tukarPoint} onChange={(e) => setTukarPoint(e.target.value)} />
          </Row>
          <Row id="tx-grand" label="Grand Total">
            <input id="tx-grand" readOnly className="num" value={money.format(grandTotal)} />
          </Row>
          <Row id="tx-sisa" label="Sisa">
            <input id="tx-sisa" readOnly className="num" value={money.format(sisa)} />
          </Row>
          <Row id="tx-kembali" label="Kembalian">
            <input id="tx-kembali" readOnly className="num" value={money.format(kembalian)} />
          </Row>
        </Card>
      </div>
      {lookupOpen && <PendaftaranLookup cabang={cabang} options={options} onPick={pick} onClose={() => setLookupOpen(false)} />}
      {dokterOpen && (
        <DokterLookup
          cabang={cabang}
          branchName={options.branches.find((b) => b.kode === cabang)?.nama ?? cabang}
          onPick={(d) => {
            setDokter({ id: d.id, nama: d.nama });
            setDokterOpen(false);
          }}
          onClose={() => setDokterOpen(false)}
        />
      )}
      {obatOpen && (
        <ObatLookup
          cabang={cabang}
          branchName={options.branches.find((b) => b.kode === cabang)?.nama ?? cabang}
          inList={new Map(obat.map((l) => [l.idObat, l.byk]))}
          onPick={addObat}
          onClose={() => setObatOpen(false)}
        />
      )}
    </>
  );
}
