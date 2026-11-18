import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Building2, ListTree, X } from "lucide-react";
import { api, ApiError, BranchOption, MenuItem, UserWithRole } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import MenuTree, { countPages } from "../components/MenuTree";
import { useLayout } from "../layout/AppLayout";

type RowStatus = { kind: "saving" | "saved" | "error"; text: string };

/** Management > User: give each user a role and the branches they belong to. */
export default function UserPage() {
  const { token, logout } = useAuth();
  const { me } = useLayout();
  const [users, setUsers] = useState<UserWithRole[] | null>(null);
  const [roles, setRoles] = useState<{ id: number; name: string }[]>([]);
  const [branches, setBranches] = useState<BranchOption[]>([]);
  const [loadError, setLoadError] = useState("");
  const [rowStatus, setRowStatus] = useState<Record<string, RowStatus>>({});
  const [editingBranches, setEditingBranches] = useState<UserWithRole | null>(null);
  const [viewingMenus, setViewingMenus] = useState<UserWithRole | null>(null);

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .users(token)
      .then((res) => {
        setUsers(res.users);
        setRoles(res.roles);
        setBranches(res.branches);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load users");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const updateUser = (id: string, patch: Partial<UserWithRole>) =>
    setUsers((list) => list?.map((u) => (u.id === id ? { ...u, ...patch } : u)) ?? null);

  async function changeRole(user: UserWithRole, value: string) {
    if (!token) return;
    const roleId = value === "" ? null : Number(value);
    setRowStatus((s) => ({ ...s, [user.id]: { kind: "saving", text: "Saving…" } }));
    try {
      await api.setUserRole(token, user.id, roleId);
      updateUser(user.id, { roleId });
      setRowStatus((s) => ({ ...s, [user.id]: { kind: "saved", text: "Saved" } }));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) expired();
      const text = err instanceof Error ? err.message : "Failed to save";
      setRowStatus((s) => ({ ...s, [user.id]: { kind: "error", text } }));
    }
  }

  const roleName = (id: number | null) => roles.find((r) => r.id === id)?.name ?? "No role";
  const branchName = useMemo(() => new Map(branches.map((b) => [b.kode, b.nama])), [branches]);

  const columns: Column<UserWithRole>[] = [
    {
      key: "email",
      header: "Email",
      value: (u) => u.email,
      render: (u) => (
        <>
          {u.email}
          {u.id === me?.id && <span className="muted"> (you)</span>}
        </>
      ),
    },
    {
      key: "role",
      header: "Role",
      value: (u) => roleName(u.roleId),
      render: (u) => {
        const self = u.id === me?.id;
        const st = rowStatus[u.id];
        return (
          <div className="role-cell">
            <select
              className="select"
              aria-label={`Role for ${u.email}`}
              value={u.roleId ?? ""}
              disabled={self || st?.kind === "saving"}
              title={self ? "You cannot change your own role" : undefined}
              onChange={(e) => changeRole(u, e.target.value)}
            >
              <option value="">No role</option>
              {roles.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
            <span aria-live="polite" className={st?.kind === "error" ? "row-status error" : "row-status"}>
              {st?.text}
            </span>
          </div>
        );
      },
    },
    {
      key: "branches",
      header: "Cabang",
      value: (u) => u.branches.join(", "),
      render: (u) => (
        <div className="branch-cell">
          {u.branches.length === 0 ? (
            <span className="muted">No branch</span>
          ) : (
            <ul className="chips" aria-label={`Branches of ${u.email}`}>
              {u.branches.map((k) => (
                <li key={k} className="chip" title={branchName.get(k) ?? k}>
                  {k}
                </li>
              ))}
            </ul>
          )}
          <button
            type="button"
            className="row-action"
            aria-label={`Set branches for ${u.email}`}
            title="Set branches"
            onClick={() => setEditingBranches(u)}
          >
            <Building2 aria-hidden="true" />
          </button>
        </div>
      ),
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (u) => (
        <button
          type="button"
          className="row-action"
          aria-label={`View menus of ${u.email}`}
          title="View menus"
          onClick={() => setViewingMenus(u)}
        >
          <ListTree aria-hidden="true" />
        </button>
      ),
    },
  ];

  if (loadError) {
    return (
      <section className="panel">
        <h1>User</h1>
        <p className="alert error" role="alert">
          {loadError}
        </p>
      </section>
    );
  }
  if (!users) return <p className="muted">Loading…</p>;

  return (
    <section className="panel">
      <h1>User</h1>
      <p className="muted">
        Pick a role for each user and the branches they work at; a user can belong to several branches. A user without a
        role sees no menus. Roles are edited under Management › Role.
      </p>
      <DataTable title="User" columns={columns} rows={users} rowKey={(u) => u.id} initialSort={{ key: "email", dir: "asc" }} />
      {viewingMenus && (
        <UserMenusDialog
          user={viewingMenus}
          roleName={roleName(viewingMenus.roleId)}
          onClose={() => setViewingMenus(null)}
          onUnauthorized={expired}
        />
      )}
      {editingBranches && (
        <BranchPicker
          user={editingBranches}
          options={branches}
          onClose={() => setEditingBranches(null)}
          onUnauthorized={expired}
          onSaved={(codes) => {
            updateUser(editingBranches.id, { branches: codes });
            setRowStatus((s) => ({ ...s, [editingBranches.id]: { kind: "saved", text: "Branches saved" } }));
            setEditingBranches(null);
          }}
        />
      )}
    </section>
  );
}

function BranchPicker({
  user,
  options,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  user: UserWithRole;
  options: BranchOption[];
  onClose: () => void;
  onSaved: (codes: string[]) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [selected, setSelected] = useState<Set<string>>(() => new Set(user.branches));
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const d = dialogRef.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  const q = filter.trim().toLowerCase();
  const shown = q ? options.filter((b) => `${b.kode} ${b.nama}`.toLowerCase().includes(q)) : options;

  const toggle = (kode: string) =>
    setSelected((s) => {
      const next = new Set(s);
      if (next.has(kode)) next.delete(kode);
      else next.add(kode);
      return next;
    });
  const setShown = (on: boolean) =>
    setSelected((s) => {
      const next = new Set(s);
      for (const b of shown) {
        if (on) next.add(b.kode);
        else next.delete(b.kode);
      }
      return next;
    });

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      const res = await api.setUserBranches(token, user.id, [...selected]);
      onSaved(res.branches);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else setError(err instanceof ApiError ? (err.fieldErrors.branches ?? err.message) : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialogRef} className="modal" aria-labelledby="branch-picker-title" onClose={onClose}>
      <form onSubmit={save}>
        <header className="modal-head">
          <h2 id="branch-picker-title">Cabang · {user.email}</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">
          <div className="picker-tools">
            <input
              type="search"
              className="picker-search"
              placeholder="Filter branches"
              aria-label="Filter branches"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
            <button type="button" className="btn-small" onClick={() => setShown(true)}>
              Select all
            </button>
            <button type="button" className="btn-small" onClick={() => setShown(false)}>
              Clear
            </button>
          </div>
          <p className="muted picker-count" aria-live="polite">
            {selected.size} of {options.length} selected
          </p>
          <fieldset className="picker-list">
            <legend className="visually-hidden">Branches</legend>
            {shown.length === 0 && <p className="muted">No branch matches “{filter}”.</p>}
            {shown.map((b) => (
              <label key={b.kode} className="picker-item">
                <input type="checkbox" checked={selected.has(b.kode)} onChange={() => toggle(b.kode)} />
                <span className="picker-kode">{b.kode}</span>
                <span className="picker-nama">{b.nama}</span>
                {!b.isActive && <span className="status off">Nonaktif</span>}
              </label>
            ))}
          </fieldset>
        </div>
        {error && (
          <p className="alert error modal-alert" role="alert">
            {error}
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

/** Read-only view of the menus a user gets from their role, as a tree. */
function UserMenusDialog({
  user,
  roleName,
  onClose,
  onUnauthorized,
}: {
  user: UserWithRole;
  roleName: string;
  onClose: () => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [data, setData] = useState<{ role: string | null; allAccess: boolean; menus: MenuItem[] } | null>(null);
  const [error, setError] = useState("");
  // Changing the key remounts the tree with every group open or closed.
  const [tree, setTree] = useState({ key: 0, open: true });

  useEffect(() => {
    const d = dialogRef.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  useEffect(() => {
    if (!token) return;
    api
      .userMenus(token, user.id)
      .then(setData)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) onUnauthorized();
        else setError(err instanceof Error ? err.message : "Failed to load menus");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const pages = data ? data.menus.reduce((n, m) => n + countPages(m), 0) : 0;

  return (
    <dialog ref={dialogRef} className="modal modal-tree" aria-labelledby="user-menus-title" onClose={onClose}>
      <header className="modal-head">
        <div>
          <h2 id="user-menus-title">{data?.role ?? roleName}</h2>
          <p className="muted modal-sub">{user.email}</p>
        </div>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body tree-body">
        {error ? (
          <p className="alert error" role="alert">
            {error}
          </p>
        ) : !data ? (
          <p className="muted">Loading…</p>
        ) : data.menus.length === 0 ? (
          <p className="muted">{data.role ? "This role has no menus." : "This user has no role, so they see no menus."}</p>
        ) : (
          <>
            <div className="tree-tools">
              <span className="muted">
                {data.allAccess ? "All menus · " : ""}
                {pages} page{pages === 1 ? "" : "s"}
              </span>
              <button type="button" className="btn-small" onClick={() => setTree((t) => ({ key: t.key + 1, open: true }))}>
                Expand all
              </button>
              <button type="button" className="btn-small" onClick={() => setTree((t) => ({ key: t.key + 1, open: false }))}>
                Collapse all
              </button>
            </div>
            <MenuTree key={tree.key} items={data.menus} defaultOpen={tree.open} />
          </>
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
