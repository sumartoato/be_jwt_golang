import { CSSProperties, useEffect, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { ChevronDown } from "lucide-react";
import type { MenuItem } from "../api";
import { iconFor } from "./icons";
import { findTrail } from "./menuUtils";

interface SidebarProps {
  menus: MenuItem[] | null;
  error: string;
  onRetry: () => void;
}

export default function Sidebar({ menus, error, onRetry }: SidebarProps) {
  const { pathname } = useLocation();
  const [open, setOpen] = useState<Set<number>>(new Set());

  // Expand the groups that contain the current page.
  useEffect(() => {
    if (!menus) return;
    const ancestors = findTrail(menus, pathname).slice(0, -1);
    if (ancestors.length === 0) return;
    setOpen((prev) => {
      if (ancestors.every((a) => prev.has(a.id))) return prev;
      const next = new Set(prev);
      ancestors.forEach((a) => next.add(a.id));
      return next;
    });
  }, [menus, pathname]);

  const toggle = (id: number) =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  if (error) {
    return (
      <div className="sidebar-state">
        <p className="alert error">{error}</p>
        <button type="button" className="btn-small" onClick={onRetry}>
          Try again
        </button>
      </div>
    );
  }
  if (!menus) return <p className="sidebar-state muted">Loading menu…</p>;
  if (menus.length === 0) return <p className="sidebar-state muted">No menus for your role yet. Ask an admin to assign one.</p>;

  return (
    <nav aria-label="Main menu">
      <ul className="nav-list">
        {menus.map((item) => (
          <MenuNode key={item.id} item={item} depth={0} open={open} toggle={toggle} />
        ))}
      </ul>
    </nav>
  );
}

interface MenuNodeProps {
  item: MenuItem;
  depth: number;
  open: Set<number>;
  toggle: (id: number) => void;
}

function MenuNode({ item, depth, open, toggle }: MenuNodeProps) {
  const Icon = iconFor(item.icon);
  const style = { "--depth": depth } as CSSProperties;
  const content = (
    <>
      <Icon className="nav-icon" aria-hidden="true" />
      <span className="nav-title">{item.title}</span>
    </>
  );

  if (item.children.length > 0) {
    const isOpen = open.has(item.id);
    const subId = `nav-sub-${item.id}`;
    return (
      <li className={isOpen ? "nav-group open" : "nav-group"}>
        <button type="button" className="nav-link" style={style} aria-expanded={isOpen} aria-controls={subId} onClick={() => toggle(item.id)}>
          {content}
          <ChevronDown className="nav-chevron" aria-hidden="true" />
        </button>
        {isOpen && (
          <ul id={subId} className="nav-list nav-sub">
            {item.children.map((child) => (
              <MenuNode key={child.id} item={child} depth={depth + 1} open={open} toggle={toggle} />
            ))}
          </ul>
        )}
      </li>
    );
  }

  if (item.path) {
    return (
      <li>
        <NavLink to={item.path} end className="nav-link" style={style}>
          {content}
        </NavLink>
      </li>
    );
  }

  // No page and no children: show it, but it does nothing.
  return (
    <li>
      <span className="nav-link disabled" style={style} aria-disabled="true">
        {content}
      </span>
    </li>
  );
}
