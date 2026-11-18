import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, Distributor, DistributorInput, FieldErrors } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data Klinik > Distributor: drug distributors (refdistributorobat). */
export default function DistributorPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<Distributor[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Distributor | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .distributor(token)
      .then((res) => setList(res.distributor))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load distributors");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<Distributor>[] = [
    { key: "id", header: "Kode", value: (d) => String(d.id), sortValue: (d) => d.id },
    { key: "nama", header: "Nama", value: (d) => d.nama, className: "dt-wrap" },
    { key: "alamat", header: "Alamat", value: (d) => orDash(d.alamat), className: "dt-wrap" },
    { key: "telp", header: "Telp", value: (d) => orDash(d.telp) },
    { key: "fax", header: "Fax", value: (d) => orDash(d.fax) },
    { key: "cp", header: "Kontak Person", value: (d) => orDash(d.cp) },
    { key: "telpCp", header: "Telp KP", value: (d) => orDash(d.telpCp) },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <button type="button" className="row-action" aria-label={`Edit ${d.nama}`} title="Edit" onClick={() => setEditing(d)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Distributor</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list}>
          <Plus aria-hidden="true" /> Add
        </button>
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
      ) : (
        <DataTable title="Distributor" columns={columns} rows={list} rowKey={(d) => d.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <DistributorForm
          distributor={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(d, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), d] : (l ?? []).map((x) => (x.id === d.id ? d : x))));
            setEditing(null);
            setSaved(`Distributor ${d.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

type Key = keyof DistributorInput;

function DistributorForm({
  distributor,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  distributor: Distributor | null;
  onClose: () => void;
  onSaved: (d: Distributor, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<DistributorInput>(() => ({
    nama: distributor?.nama ?? "",
    alamat: distributor?.alamat ?? "",
    telp: distributor?.telp ?? "",
    fax: distributor?.fax ?? "",
    cp: distributor?.cp ?? "",
    telpCp: distributor?.telpCp ?? "",
  }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!form.nama.trim()) {
      setErrors({ nama: "Nama is required" });
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = distributor ? await api.updateDistributor(token, distributor.id, form) : await api.createDistributor(token, form);
      onSaved(saved, !distributor);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  // max matches the varchar size of the refdistributorobat column.
  const field = (key: Key, label: string, max: number, extra: { required?: boolean; wide?: boolean } = {}) => (
    <FieldShell id={`distributor-${key}`} label={label} error={errors[key]} required={extra.required} wide={extra.wide}>
      <input
        id={`distributor-${key}`}
        value={form[key]}
        maxLength={max}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => {
          setForm((f) => ({ ...f, [key]: e.target.value }));
          setErrors(({ [key]: _, ...rest }) => rest);
        }}
      />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={distributor ? `Edit Distributor #${distributor.id}` : "Add Distributor"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        {field("nama", "Nama", 300, { required: true, wide: true })}
        {field("alamat", "Alamat", 200, { wide: true })}
        {field("telp", "Telp", 15)}
        {field("fax", "Fax", 25)}
        {field("cp", "Kontak Person", 40)}
        {field("telpCp", "Telp KP", 25)}
      </div>
    </Modal>
  );
}
