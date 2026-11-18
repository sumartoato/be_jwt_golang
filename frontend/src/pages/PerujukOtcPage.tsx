import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, PerujukOtc } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data Klinik > Penjual OTC: people who refer over-the-counter sales (refperujukotc). */
export default function PerujukOtcPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<PerujukOtc[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<PerujukOtc | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .perujukOtc(token)
      .then((res) => setList(res.perujuk))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<PerujukOtc>[] = [
    { key: "id", header: "ID", value: (p) => String(p.id), sortValue: (p) => p.id },
    { key: "nama", header: "Nama", value: (p) => p.nama, className: "dt-wrap dt-grow" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (p) => (
        <button type="button" className="row-action" aria-label={`Edit ${p.nama}`} title="Edit" onClick={() => setEditing(p)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Perujuk OTC</h1>
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
        <DataTable title="Perujuk OTC" columns={columns} rows={list} rowKey={(p) => p.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <PerujukForm
          perujuk={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(p, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), p] : (l ?? []).map((x) => (x.id === p.id ? p : x))));
            setEditing(null);
            setSaved(`${p.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function PerujukForm({
  perujuk,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  perujuk: PerujukOtc | null;
  onClose: () => void;
  onSaved: (p: PerujukOtc, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [nama, setNama] = useState(perujuk?.nama ?? "");
  const [nameError, setNameError] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!nama.trim()) {
      setNameError("Nama is required");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = perujuk ? await api.updatePerujukOtc(token, perujuk.id, nama) : await api.createPerujukOtc(token, nama);
      onSaved(saved, !perujuk);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && err.fieldErrors.nama) setNameError(err.fieldErrors.nama);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={perujuk ? `Edit Perujuk OTC #${perujuk.id}` : "Add Perujuk OTC"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <FieldShell id="perujuk-nama" label="Nama" error={nameError} required>
        <input
          id="perujuk-nama"
          value={nama}
          maxLength={50}
          aria-invalid={nameError ? "true" : undefined}
          onChange={(e) => {
            setNama(e.target.value);
            setNameError("");
          }}
        />
      </FieldShell>
    </Modal>
  );
}
