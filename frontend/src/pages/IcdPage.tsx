import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, Icd, IcdInput, LookupOption } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";

const EXPORT_LIMIT = 5000;

/** Master Data Klinik > ICD: ~18.5k ICD-X codes, paged on the server. */
export default function IcdPage() {
  const { token, logout } = useAuth();
  const [jenis, setJenis] = useState<LookupOption[] | null>(null);
  const [page, setPage] = useState<{ rows: Icd[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Icd | "new" | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "info" | "error"; text: string } | null>(null);
  const lastQuery = useRef<RemoteQuery | null>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .icdOptions(token)
      .then((o) => setJenis(o.jenis))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page) => ({
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      lastQuery.current = q;
      const mine = ++seq.current;
      setLoading(true);
      api
        .icd(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof Error ? err.message : "Failed to load ICD codes" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<Icd[]> => {
    if (!token) return [];
    const res = await api.icd(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} matching codes. Narrow the search to export the rest.`,
      });
    }
    return res.rows;
  };

  const columns: Column<Icd>[] = [
    { key: "id", header: "Kode", value: (x) => String(x.id) },
    { key: "kode", header: "Kode ICD-X", value: (x) => x.kode },
    { key: "nama", header: "Nama Penyakit", value: (x) => x.nama, className: "dt-wrap dt-grow" },
    { key: "jenis", header: "Jenis ICD-X", value: (x) => x.jenis || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (x) => (
        <button type="button" className="row-action" aria-label={`Edit ${x.kode}`} title="Edit" onClick={() => setEditing(x)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>ICD</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!jenis}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !jenis ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable
          title="ICD"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(x) => x.id}
          initialSort={{ key: "id", dir: "asc" }}
          defaultPageSize={10}
          remote={{
            total: page?.total ?? 0,
            unfilteredTotal: page?.unfilteredTotal ?? 0,
            loading: loading || !page,
            onQuery: fetchPage,
            fetchAll,
          }}
        />
      )}
      {editing && jenis && (
        <IcdForm
          icd={editing === "new" ? null : editing}
          jenis={jenis}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(x, isNew) => {
            setEditing(null);
            setNotice({ kind: "success", text: `ICD ${x.kode} ${isNew ? "added" : "updated"}.` });
            if (lastQuery.current) fetchPage(lastQuery.current);
          }}
        />
      )}
    </section>
  );
}

function IcdForm({
  icd,
  jenis,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  icd: Icd | null;
  jenis: LookupOption[];
  onClose: () => void;
  onSaved: (x: Icd, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<IcdInput>(() => ({ kode: icd?.kode ?? "", nama: icd?.nama ?? "", idJenis: icd?.idJenis ?? null }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const update = <K extends keyof IcdInput>(key: K, value: IcdInput[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const local: FieldErrors = {};
    if (!form.kode.trim()) local.kode = "Kode ICD-X is required";
    if (!form.nama.trim()) local.nama = "Nama Penyakit is required";
    if (form.idJenis === null) local.idJenis = "Jenis ICD-X is required";
    setErrors(local);
    if (Object.keys(local).length) return;
    setBusy(true);
    setError("");
    try {
      const saved = icd ? await api.updateIcd(token, icd.id, form) : await api.createIcd(token, form);
      onSaved(saved, !icd);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={icd ? `Edit ICD #${icd.id}` : "Add ICD"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="icd-kode" label="Kode ICD-X" error={errors.kode} required>
          <input id="icd-kode" value={form.kode} maxLength={10} aria-invalid={errors.kode ? "true" : undefined} onChange={(e) => update("kode", e.target.value)} />
        </FieldShell>
        <FieldShell id="icd-idJenis" label="Jenis ICD-X" error={errors.idJenis} required>
          <select id="icd-idJenis" className="select" value={form.idJenis ?? ""} onChange={(e) => update("idJenis", e.target.value === "" ? null : Number(e.target.value))}>
            <option value="">Choose…</option>
            {jenis.map((j) => (
              <option key={j.id} value={j.id}>
                {j.name}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="icd-nama" label="Nama Penyakit" error={errors.nama} required wide>
          <input id="icd-nama" value={form.nama} maxLength={250} aria-invalid={errors.nama ? "true" : undefined} onChange={(e) => update("nama", e.target.value)} />
        </FieldShell>
      </div>
    </Modal>
  );
}
