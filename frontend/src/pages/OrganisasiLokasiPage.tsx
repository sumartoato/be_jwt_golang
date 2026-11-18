import { FormEvent, useEffect, useState } from "react";
import { Pencil } from "lucide-react";
import { api, ApiError, FieldErrors, OrganisasiLokasi, OrganisasiLokasiInput } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";
import { NoBranchNotice } from "../components/branchView";
import { fmtDate } from "../components/dates";

/**
 * Master Data Satu Sehat > Organisasi & Lokasi: per branch and poli, the
 * SATUSEHAT Organization and Location (reforganizationlocation). Rows are fixed
 * per branch and poli, so they can only be edited, and only for the user's branches.
 */
export default function OrganisasiLokasiPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<OrganisasiLokasi[] | null>(null);
  const [hasBranches, setHasBranches] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<OrganisasiLokasi | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .organisasiLokasi(token)
      .then((res) => {
        setList(res.rows);
        setHasBranches(res.branches.length > 0);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<OrganisasiLokasi>[] = [
    { key: "kode", header: "Kode", value: (r) => r.cabang },
    { key: "cabang", header: "Cabang", value: (r) => r.namaCabang, className: "dt-wrap" },
    { key: "poli", header: "Poli", value: (r) => r.poli, sortValue: (r) => r.idJenisRawat },
    { key: "organisasi", header: "Organisasi", value: (r) => r.organisasi, className: "dt-wrap" },
    { key: "lokasi", header: "Lokasi", value: (r) => r.lokasi, className: "dt-wrap" },
    { key: "organisasiId", header: "Organisasi ID", value: (r) => r.organisasiId },
    { key: "lokasiId", header: "Lokasi ID", value: (r) => r.lokasiId },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (r) => (
        <button
          type="button"
          className="row-action"
          aria-label={`Edit ${r.cabang} ${r.poli}`}
          title="Edit"
          onClick={() => {
            setSaved("");
            setEditing(r);
          }}
        >
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Organisasi &amp; Lokasi</h1>
      </div>
      {saved && (
        <p className="alert success" role="status">
          {saved}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !list ? (
        <p className="muted">Loading…</p>
      ) : !hasBranches ? (
        <NoBranchNotice what="organizations and locations" />
      ) : (
        <DataTable
          title="Organisasi & Lokasi"
          columns={columns}
          rows={list}
          rowKey={(r) => r.id}
          initialSort={{ key: "cabang", dir: "asc" }}
          defaultPageSize={10}
        />
      )}
      {editing && (
        <OrganisasiLokasiForm
          row={editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(r) => {
            setList((l) => (l ?? []).map((x) => (x.id === r.id ? r : x)));
            setEditing(null);
            setSaved(`${r.cabang} · ${r.poli} updated.`);
          }}
        />
      )}
    </section>
  );
}

/** "2024-06-20T08:52:59.313Z" (clinic time stored without zone) -> "20/06/2024 08:52". */
const fmtStamp = (s: string | null) => (s ? `${fmtDate(s)} ${s.slice(11, 16)}` : "");

function OrganisasiLokasiForm({
  row,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  row: OrganisasiLokasi;
  onClose: () => void;
  onSaved: (r: OrganisasiLokasi) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<OrganisasiLokasiInput>({
    organisasi: row.organisasi,
    lokasi: row.lokasi,
    organisasiId: row.organisasiId,
    lokasiId: row.lokasiId,
  });
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const set = (k: keyof OrganisasiLokasiInput, v: string) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors((e) => ({ ...e, [k]: "" }));
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const local: FieldErrors = {};
    if (!form.organisasi.trim()) local.organisasi = "Organisasi is required";
    if (!form.lokasi.trim()) local.lokasi = "Lokasi is required";
    if (Object.keys(local).length) {
      setErrors(local);
      return;
    }
    setBusy(true);
    setError("");
    try {
      onSaved(await api.updateOrganisasiLokasi(token, row.id, form));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const readOnly = (id: string, label: string, value: string) => (
    <FieldShell id={id} label={label}>
      <input id={id} value={value} readOnly tabIndex={-1} className="readonly" />
    </FieldShell>
  );

  const text = (k: keyof OrganisasiLokasiInput, label: string, opts: { required?: boolean; maxLength?: number; hint?: string; wide?: boolean }) => {
    const id = `orglok-${k}`;
    return (
      <FieldShell id={id} label={label} error={errors[k]} hint={opts.hint} required={opts.required} wide={opts.wide}>
        <input
          id={id}
          value={form[k]}
          maxLength={opts.maxLength}
          spellCheck={false}
          aria-invalid={errors[k] ? "true" : undefined}
          aria-describedby={errors[k] ? `${id}-error` : undefined}
          onChange={(e) => set(k, e.target.value)}
        />
      </FieldShell>
    );
  };

  const updated = row.updatedBy || row.updatedAt ? `Last updated ${fmtStamp(row.updatedAt)}${row.updatedBy ? ` by ${row.updatedBy}` : ""}`.trim() : "";

  return (
    <Modal dialogRef={dialogRef} title={`Edit Organisasi & Lokasi · ${row.cabang} ${row.poli}`} onClose={onClose} onSubmit={submit} error={error} busy={busy} wide>
      <div className="form-grid">
        {readOnly("orglok-kode", "Kode", row.cabang)}
        {readOnly("orglok-cabang", "Cabang", row.namaCabang)}
        {readOnly("orglok-poli", "Poli", row.poli || "-")}
        <span aria-hidden="true" />
        {text("organisasi", "Organisasi", { required: true, maxLength: 50 })}
        {text("lokasi", "Lokasi", { required: true, maxLength: 50 })}
        {text("organisasiId", "Organisasi ID (SATUSEHAT)", { maxLength: 36, wide: true, hint: "UUID from SATUSEHAT; leave empty until registered." })}
        {text("lokasiId", "Lokasi ID (SATUSEHAT)", { maxLength: 36, wide: true, hint: "UUID from SATUSEHAT; leave empty until registered." })}
        {updated && <p className="muted wide-note">{updated}</p>}
      </div>
    </Modal>
  );
}
