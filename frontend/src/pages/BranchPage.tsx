import { FormEvent, useEffect, useRef, useState } from "react";
import { Eye, EyeOff, Pencil, Plus, X } from "lucide-react";
import { api, ApiError, Branch, BranchInput, FieldErrors } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";

const EMPTY: BranchInput = {
  kode: "",
  nama: "",
  alamat: "",
  telp: "",
  kodeKwt: "",
  npwp: "",
  namaNpwp: "",
  dbHbee: "",
  kodeFasyankes: "",
  namaFasyankes: "",
  clientKey: "",
  secretKey: "",
  organizationId: "",
  isActive: true,
  isInternal: true,
};

function toInput(b: Branch): BranchInput {
  const { id: _id, secretKeySet: _set, secretKeyHint: _hint, ...rest } = b;
  return { ...rest, secretKey: "" };
}

const maskedSecret = (b: Branch) => (b.secretKeySet ? `••••••••${b.secretKeyHint}` : "");

/** Management > Cabang: rows of refbranch with their Satu Sehat settings. */
export default function BranchPage() {
  const { token, logout } = useAuth();
  const [branches, setBranches] = useState<Branch[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Branch | "new" | null>(null);
  const [saved, setSaved] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .branches(token)
      .then((res) => setBranches(res.branches))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setLoadError(err instanceof Error ? err.message : "Failed to load branches");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const onSaved = (b: Branch, isNew: boolean) => {
    setBranches((list) => (isNew ? [...(list ?? []), b] : (list ?? []).map((x) => (x.kode === b.kode ? b : x))));
    setSaved(`Cabang ${b.kode} ${isNew ? "added" : "updated"}.`);
    setEditing(null);
  };

  // Columns from the reference screen are shown; the other refbranch columns
  // start hidden and can be turned on under "Column visibility".
  const columns: Column<Branch>[] = [
    { key: "kode", header: "Kode Cabang", value: (b) => b.kode },
    { key: "nama", header: "Nama Cabang", value: (b) => b.nama },
    { key: "kodeKwt", header: "Kode KWT", value: (b) => b.kodeKwt, hidden: true },
    { key: "alamat", header: "Alamat", value: (b) => b.alamat, hidden: true },
    { key: "telp", header: "Telp", value: (b) => b.telp, hidden: true },
    { key: "npwp", header: "NPWP", value: (b) => b.npwp, hidden: true },
    { key: "namaNpwp", header: "Nama NPWP", value: (b) => b.namaNpwp, hidden: true },
    { key: "kodeFasyankes", header: "Kode Fasyankes", value: (b) => b.kodeFasyankes },
    { key: "namaFasyankes", header: "Nama Fasyankes", value: (b) => b.namaFasyankes },
    { key: "clientKey", header: "Client Key", value: (b) => b.clientKey, className: "dt-key" },
    { key: "secretKey", header: "Secret Key", value: maskedSecret, className: "dt-key", sortable: false },
    { key: "organizationId", header: "Organization ID", value: (b) => b.organizationId },
    { key: "dbHbee", header: "DB Hbee", value: (b) => b.dbHbee },
    { key: "internal", header: "Internal", value: (b) => (b.isInternal ? "Ya" : "Tidak"), hidden: true },
    {
      key: "status",
      header: "Status",
      value: (b) => (b.isActive ? "Aktif" : "Nonaktif"),
      render: (b) => <span className={b.isActive ? "status on" : "status off"}>{b.isActive ? "Aktif" : "Nonaktif"}</span>,
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (b) => (
        <button type="button" className="row-action" aria-label={`Edit ${b.nama || b.kode}`} title="Edit" onClick={() => setEditing(b)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Cabang</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")}>
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
      ) : !branches ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable
          title="Cabang"
          columns={columns}
          rows={branches}
          rowKey={(b) => b.kode}
          initialSort={{ key: "nama", dir: "asc" }}
        />
      )}
      {editing && (
        <BranchForm
          branch={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={onSaved}
          onUnauthorized={() => logout("Your session has expired. Please log in again.")}
        />
      )}
    </section>
  );
}

type TextKey = Exclude<keyof BranchInput, "isActive" | "isInternal" | "secretKey">;

function BranchForm({
  branch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  branch: Branch | null;
  onClose: () => void;
  onSaved: (b: Branch, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [form, setForm] = useState<BranchInput>(() => (branch ? toInput(branch) : EMPTY));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState("");
  const [showSecret, setShowSecret] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const d = dialogRef.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  const set = (key: keyof BranchInput) => (e: { target: { value: string } }) => {
    setForm((f) => ({ ...f, [key]: e.target.value }));
    // Clear the field's error once the user edits it.
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const local: FieldErrors = {};
    if (!branch && !form.kode.trim()) local.kode = "Kode Cabang is required";
    if (!form.nama.trim()) local.nama = "Nama Cabang is required";
    setErrors(local);
    setFormError("");
    if (Object.keys(local).length) return;

    setBusy(true);
    try {
      const saved = branch ? await api.updateBranch(token, branch.kode, form) : await api.createBranch(token, form);
      onSaved(saved, !branch);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setFormError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  // max matches the varchar size of the refbranch column.
  const field = (
    key: TextKey,
    label: string,
    opts: { max: number; required?: boolean; wide?: boolean; multiline?: boolean; disabled?: boolean; hint?: string },
  ) => {
    const id = `branch-${key}`;
    const describedBy = errors[key] ? `${id}-error` : opts.hint ? `${id}-hint` : undefined;
    const common = {
      id,
      value: form[key],
      onChange: set(key),
      maxLength: opts.max,
      required: opts.required,
      disabled: opts.disabled,
      "aria-invalid": errors[key] ? ("true" as const) : undefined,
      "aria-describedby": describedBy,
      autoComplete: "off",
    };
    return (
      <div className={opts.wide ? "field wide" : "field"}>
        <label htmlFor={id}>
          {label}
          {opts.required && <span aria-hidden="true"> *</span>}
        </label>
        {opts.multiline ? <textarea rows={2} {...common} /> : <input {...common} />}
        {errors[key] ? (
          <span id={`${id}-error`} className="field-error">
            {errors[key]}
          </span>
        ) : (
          opts.hint && (
            <span id={`${id}-hint`} className="field-hint">
              {opts.hint}
            </span>
          )
        )}
      </div>
    );
  };

  const yesNo = (key: "isActive" | "isInternal", label: string, yes: string, no: string) => (
    <div className="field">
      <label htmlFor={`branch-${key}`}>{label}</label>
      <select
        id={`branch-${key}`}
        className="select"
        value={form[key] ? "1" : "0"}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value === "1" }))}
      >
        <option value="1">{yes}</option>
        <option value="0">{no}</option>
      </select>
    </div>
  );

  return (
    <dialog ref={dialogRef} className="modal modal-wide" aria-labelledby="branch-form-title" onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <header className="modal-head">
          <h2 id="branch-form-title">{branch ? `Edit Cabang ${branch.kode}` : "Add Cabang"}</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>

        <div className="modal-body">
          <fieldset className="form-section">
            <legend>Data Cabang</legend>
            <div className="form-grid">
              {field("kode", "Kode Cabang", {
                max: 20,
                required: !branch,
                disabled: !!branch,
                hint: branch ? "Kode cannot be changed." : undefined,
              })}
              {field("nama", "Nama Cabang", { max: 50, required: true })}
              {field("kodeKwt", "Kode KWT", { max: 10 })}
              {field("telp", "Telp", { max: 200 })}
              {field("alamat", "Alamat", { max: 200, wide: true, multiline: true })}
              {field("npwp", "NPWP", { max: 50 })}
              {field("namaNpwp", "Nama NPWP", { max: 100 })}
              {yesNo("isActive", "Status", "Aktif", "Nonaktif")}
              {yesNo("isInternal", "Internal", "Ya", "Tidak")}
            </div>
          </fieldset>

          <fieldset className="form-section">
            <legend>Satu Sehat &amp; Integrasi</legend>
            <div className="form-grid">
              {field("kodeFasyankes", "Kode Fasyankes", { max: 50 })}
              {field("namaFasyankes", "Nama Fasyankes", { max: 100 })}
              {field("organizationId", "Organization ID", { max: 100 })}
              {field("dbHbee", "DB Hbee", { max: 50 })}
              {field("clientKey", "Client Key", { max: 100, wide: true })}

              <div className="field wide">
                <label htmlFor="branch-secretKey">Secret Key</label>
                <div className="input-with-btn">
                  <input
                    id="branch-secretKey"
                    type={showSecret ? "text" : "password"}
                    value={form.secretKey}
                    onChange={set("secretKey")}
                    maxLength={100}
                    autoComplete="new-password"
                    placeholder={branch?.secretKeySet ? `Leave blank to keep the current key (••••${branch.secretKeyHint})` : ""}
                    aria-invalid={errors.secretKey ? "true" : undefined}
                    aria-describedby="branch-secretKey-hint"
                  />
                  <button
                    type="button"
                    className="row-action"
                    aria-label={showSecret ? "Hide secret key" : "Show secret key"}
                    aria-pressed={showSecret}
                    onClick={() => setShowSecret((s) => !s)}
                  >
                    {showSecret ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                  </button>
                </div>
                <span id="branch-secretKey-hint" className={errors.secretKey ? "field-error" : "field-hint"}>
                  {errors.secretKey ?? "Stored on the server and never shown again; only the last 4 characters are displayed."}
                </span>
              </div>
            </div>
          </fieldset>
        </div>

        {formError && (
          <p className="alert error modal-alert" role="alert">
            {formError}
          </p>
        )}

        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-small btn-primary" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}
