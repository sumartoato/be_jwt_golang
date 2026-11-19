import { useCallback, useEffect, useRef, useState } from "react";
import { Outlet, useLocation, useOutletContext } from "react-router-dom";
import { ChevronLeft, ChevronRight, LogOut, Menu } from "lucide-react";
import { api, ApiError, MenuItem, MeResponse } from "../api";
import { useAuth } from "../auth/AuthContext";
import { findPageTrail } from "./menuUtils";
import Sidebar from "./Sidebar";

export interface LayoutContext {
  me: MeResponse | null;
  /** null while loading */
  menus: MenuItem[] | null;
  /** Reloads the sidebar in the background, e.g. after a role's menus change. */
  refreshMenus: () => Promise<void>;
}

const COLLAPSED_KEY = "sidebarCollapsed";

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

export function useLayout(): LayoutContext {
  return useOutletContext<LayoutContext>();
}

/** Shell for every page after login: sidebar menu from the API, top bar, page content. */
export default function AppLayout() {
  const { token, logout } = useAuth();
  const { pathname } = useLocation();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [menus, setMenus] = useState<MenuItem[] | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [navOpen, setNavOpen] = useState(false);
  // Desktop only: hide the sidebar to give the page the full width. Remembered per browser.
  const [collapsed, setCollapsed] = useState(readCollapsed);
  const collapseBtn = useRef<HTMLButtonElement>(null);
  const expandBtn = useRef<HTMLButtonElement>(null);
  const toggledByUser = useRef(false);

  const toggleCollapsed = (value: boolean) => {
    toggledByUser.current = true;
    setCollapsed(value);
    try {
      localStorage.setItem(COLLAPSED_KEY, value ? "1" : "0");
    } catch {
      // Storage unavailable (private mode): the choice just is not remembered.
    }
  };

  // The clicked button disappears, so move focus to the one that replaces it.
  useEffect(() => {
    if (!toggledByUser.current) return;
    (collapsed ? expandBtn : collapseBtn).current?.focus();
  }, [collapsed]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    setError("");
    setMenus(null);
    Promise.all([api.me(token), api.menus(token)])
      .then(([meRes, menuRes]) => {
        if (cancelled) return;
        setMe(meRes);
        setMenus(menuRes.menus);
        setRole(menuRes.role);
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load the menu");
      });
    return () => {
      cancelled = true;
    };
    // Load once per mount (and on retry); sliding token refreshes should not refetch.
  }, [attempt]); // eslint-disable-line react-hooks/exhaustive-deps

  const refreshMenus = useCallback(async () => {
    if (!token) return;
    try {
      const res = await api.menus(token);
      setMenus(res.menus);
      setRole(res.role);
    } catch {
      // Keep the current sidebar; the next page load retries.
    }
  }, [token]);

  // Close the mobile drawer after navigating.
  useEffect(() => setNavOpen(false), [pathname]);

  useEffect(() => {
    if (!navOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setNavOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [navOpen]);

  const trail = menus ? findPageTrail(menus, pathname) : { items: [] };

  return (
    <div className={["app", navOpen && "nav-open", collapsed && "sidebar-collapsed"].filter(Boolean).join(" ")}>
      <aside className="sidebar" id="sidebar">
        <div className="sidebar-brand">
          <span>RME Cabang</span>
          <button
            ref={collapseBtn}
            type="button"
            className="icon-btn sidebar-collapse"
            aria-label="Hide menu"
            title="Hide menu"
            aria-controls="sidebar"
            aria-expanded={true}
            onClick={() => toggleCollapsed(true)}
          >
            <ChevronLeft aria-hidden="true" />
          </button>
        </div>
        <Sidebar menus={menus} error={error} onRetry={() => setAttempt((n) => n + 1)} />
      </aside>
      <div className="backdrop" onClick={() => setNavOpen(false)} aria-hidden="true" />

      <div className="main">
        <header className="topbar">
          {collapsed && (
            <button
              ref={expandBtn}
              type="button"
              className="icon-btn sidebar-expand"
              aria-label="Show menu"
              title="Show menu"
              aria-controls="sidebar"
              aria-expanded={false}
              onClick={() => toggleCollapsed(false)}
            >
              <ChevronRight aria-hidden="true" />
            </button>
          )}
          <button
            type="button"
            className="icon-btn nav-toggle"
            aria-label="Open menu"
            aria-controls="sidebar"
            aria-expanded={navOpen}
            onClick={() => setNavOpen((o) => !o)}
          >
            <Menu aria-hidden="true" />
          </button>
          <nav aria-label="Breadcrumb" className="breadcrumb">
            <ol>
              {trail.items.map((item) => (
                <li key={item.id}>{item.title}</li>
              ))}
              {trail.sub && <li>{trail.sub}</li>}
            </ol>
          </nav>
          {me && (
            <span className="topbar-user">
              {me.email}
              <span className="role-badge">{role ?? "No role"}</span>
            </span>
          )}
          <button type="button" className="icon-btn" onClick={() => logout()}>
            <LogOut aria-hidden="true" />
            <span>Log out</span>
          </button>
        </header>

        <main className="content">
          <Outlet context={{ me, menus, refreshMenus } satisfies LayoutContext} />
        </main>
      </div>
    </div>
  );
}
