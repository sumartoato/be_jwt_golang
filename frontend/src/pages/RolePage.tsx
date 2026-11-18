import { FormEvent, useEffect, useMemo, useState } from "react";
import { ChevronDown, Plus } from "lucide-react";
import { api, ApiError, MenuItem, Role, RoleInput } from "../api";
import { useAuth } from "../auth/AuthContext";
import { useLayout } from "../layout/AppLayout";

const EMPTY: RoleInput = { name: "", allAccess: false, menuIds: [] };

function descendantIds(item: MenuItem): number[] {
  return item.children.flatMap((c) => [c.id, ...descendantIds(c)]);
}

function leafCount(item: MenuItem): number {
  return item.children.length === 0 ? 1 : item.children.reduce((n, c) => n + leafCount(c), 0);
}

/** Leaves that are granted, directly or through a granted group above them. */
function grantedLeafCount(item: MenuItem, selected: Set<number>, inherited: boolean): number {
  const on = inherited || selected.has(item.id);
  if (item.children.length === 0) return on ? 1 : 0;
  return item.children.reduce((n, c) => n + grantedLeafCount(c, selected, on), 0);
}

/** Management > Role: create roles and choose which menus each one can open. */
export default function RolePage() {
  const { token, logout } = useAuth();
  const { refreshMenus } = useLayout();
  const [roles, setRoles] = useState<Role[] | null>(null);
  const [menus, setMenus] = useState<MenuItem[]>([]);
  const [loadError, setLoadError] = useState("");
  const [selectedId, setSelectedId] = useState<number | "new" | null>(null);
  const [draft, setDraft] = useState<RoleInput>(EMPTY);
  const [nameError, setNameError] = useState("");
  const [status, setStatus] = useState<{ kind: "error" | "success"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const fail = (err: unknown, fallback: string) => {
    if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
    else setStatus({ kind: "error", text: err instanceof Error ? err.message : fallback });
  };

  const load = async (select?: number) => {
    if (!token) return;
    try {
      const res = await api.roles(token);
      setRoles(res.roles);
      setMenus(res.menus);
      const pick = res.roles.find((r) => r.id === select) ?? res.roles[0];
      if (pick) choose(pick);
      else startNew();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
      else setLoadError(err instanceof Error ? err.message : "Failed to load roles");
    }
  };

  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function choose(role: Role) {
    setSelectedId(role.id);
    setDraft({ name: role.name, allAccess: role.allAccess, menuIds: role.menuIds });
    setNameError("");
    setStatus(null);
  }

  function startNew() {
    setSelectedId("new");
    setDraft(EMPTY);
    setNameError("");
    setStatus(null);
  }

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!token || selectedId === null) return;
    if (!draft.name.trim()) {
      setNameError("Name is required");
      return;
    }
    setBusy(true);
    setNameError("");
    setStatus(null);
    try {
      const saved =
        selectedId === "new" ? await api.createRole(token, draft) : await api.updateRole(token, selectedId, draft);
      await load(saved.id);
      setStatus({ kind: "success", text: `Role "${saved.name}" saved.` });
      void refreshMenus(); // the sidebar changes if this is your own role
    } catch (err) {
      if (err instanceof ApiError && err.fieldErrors.name) setNameError(err.fieldErrors.name);
      else fail(err, "Failed to save the role");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!token || typeof selectedId !== "number") return;
    const role = roles?.find((r) => r.id === selectedId);
    if (!role || !window.confirm(`Delete role "${role.name}"?`)) return;
    setBusy(true);
    setStatus(null);
    try {
      await api.deleteRole(token, role.id);
      await load();
      setStatus({ kind: "success", text: `Role "${role.name}" deleted.` });
    } catch (err) {
      fail(err, "Failed to delete the role");
    } finally {
      setBusy(false);
    }
  }

  if (loadError) {
    return (
      <section className="panel">
        <h1>Role</h1>
        <p className="alert error" role="alert">{loadError}</p>
      </section>
    );
  }
  if (!roles) return <p className="muted">Loading…</p>;

  const current = typeof selectedId === "number" ? roles.find((r) => r.id === selectedId) : undefined;

  return (
    <section className="panel">
      <h1>Role</h1>
      <p className="muted">Choose which menus each role can open. Users see the change the next time they load a page.</p>

      <div className="role-layout">
        <nav className="role-list" aria-label="Roles">
          <ul>
            {roles.map((r) => (
              <li key={r.id}>
                <button
                  type="button"
                  className={r.id === selectedId ? "role-item active" : "role-item"}
                  aria-current={r.id === selectedId ? "true" : undefined}
                  onClick={() => choose(r)}
                >
                  <span className="role-item-name">{r.name}</span>
                  <span className="role-item-meta">
                    {r.allAccess ? "All menus" : `${r.menuIds.length} menu${r.menuIds.length === 1 ? "" : "s"}`} ·{" "}
                    {r.userCount} user{r.userCount === 1 ? "" : "s"}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <button type="button" className="btn-small" onClick={startNew} aria-pressed={selectedId === "new"}>
            <Plus aria-hidden="true" size={16} /> New role
          </button>
        </nav>

        <form className="role-editor" onSubmit={save} noValidate>
          <h2>{selectedId === "new" ? "New role" : `Edit ${current?.name ?? ""}`}</h2>
          <div className="field">
            <label htmlFor="role-name">Name</label>
            <input
              id="role-name"
              value={draft.name}
              maxLength={50}
              aria-invalid={nameError ? "true" : undefined}
              aria-describedby={nameError ? "role-name-error" : undefined}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
            {nameError && <span id="role-name-error" className="field-error">{nameError}</span>}
          </div>

          <label className="check-row">
            <input
              type="checkbox"
              checked={draft.allAccess}
              onChange={(e) => setDraft({ ...draft, allAccess: e.target.checked })}
            />
            <span>
              <strong>All menus</strong>
              <span className="muted"> (includes menus added later)</span>
            </span>
          </label>

          <fieldset className="menu-checklist" disabled={draft.allAccess}>
            <legend>Menus</legend>
            <p className="muted">Ticking a group grants every item in it, including items added later.</p>
            <MenuChecklist
              items={menus}
              selected={draft.menuIds}
              all={draft.allAccess}
              onChange={(menuIds) => setDraft({ ...draft, menuIds })}
            />
          </fieldset>

          {status && (
            <p className={`alert ${status.kind}`} role={status.kind === "error" ? "alert" : "status"}>
              {status.text}
            </p>
          )}

          <div className="form-actions">
            <button type="submit" className="btn-small btn-primary" disabled={busy}>
              {busy ? "Saving…" : selectedId === "new" ? "Create role" : "Save changes"}
            </button>
            {current && (
              <button
                type="button"
                className="btn-small btn-danger"
                onClick={remove}
                disabled={busy || current.userCount > 0}
                title={current.userCount > 0 ? "Give its users another role first" : undefined}
              >
                Delete
              </button>
            )}
          </div>
        </form>
      </div>
    </section>
  );
}

function MenuChecklist({
  items,
  selected,
  all,
  onChange,
}: {
  items: MenuItem[];
  selected: number[];
  /** Role has every menu: show everything ticked. */
  all: boolean;
  onChange: (ids: number[]) => void;
}) {
  const set = useMemo(() => new Set(selected), [selected]);

  const toggle = (item: MenuItem) => {
    if (set.has(item.id)) {
      onChange(selected.filter((id) => id !== item.id));
    } else {
      // A granted group covers its children, so drop their own entries.
      const covered = new Set(descendantIds(item));
      onChange([...selected.filter((id) => !covered.has(id)), item.id]);
    }
  };

  return (
    <ul className="check-tree">
      {items.map((item) => (
        <CheckNode key={item.id} item={item} selected={set} inherited={all} onToggle={toggle} />
      ))}
    </ul>
  );
}

function CheckNode({
  item,
  selected,
  inherited,
  onToggle,
}: {
  item: MenuItem;
  selected: Set<number>;
  inherited: boolean;
  onToggle: (item: MenuItem) => void;
}) {
  const [open, setOpen] = useState(false);
  const isGroup = item.children.length > 0;
  const checked = inherited || selected.has(item.id);
  const granted = isGroup ? grantedLeafCount(item, selected, inherited) : 0;
  const total = isGroup ? leafCount(item) : 0;
  const partial = !checked && granted > 0;
  const childrenId = `check-children-${item.id}`;

  return (
    <li>
      <div className="check-node">
        {isGroup ? (
          <button
            type="button"
            className="check-toggle"
            aria-expanded={open}
            aria-controls={childrenId}
            aria-label={`${open ? "Collapse" : "Expand"} ${item.title}`}
            onClick={() => setOpen((o) => !o)}
          >
            <ChevronDown aria-hidden="true" size={14} />
          </button>
        ) : (
          <span className="check-toggle-spacer" />
        )}
        <label>
          <input
            type="checkbox"
            checked={checked}
            disabled={inherited}
            ref={(el) => {
              if (el) el.indeterminate = partial;
            }}
            onChange={() => onToggle(item)}
          />
          <span>{item.title}</span>
          {isGroup && (
            <span className="check-count">
              {granted}/{total}
            </span>
          )}
        </label>
      </div>
      {isGroup && open && (
        <ul id={childrenId} className="check-tree">
          {item.children.map((c) => (
            <CheckNode key={c.id} item={c} selected={selected} inherited={checked} onToggle={onToggle} />
          ))}
        </ul>
      )}
    </li>
  );
}
