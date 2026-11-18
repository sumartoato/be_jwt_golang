import type { ReactNode } from "react";
import { useLocation } from "react-router-dom";
import MenuPage from "../pages/MenuPage";
import { useLayout } from "./AppLayout";
import { findTrail } from "./menuUtils";

/**
 * Renders a page only when the current path is in the user's menu (i.e. their
 * role grants it); otherwise shows "Page not available". The backend enforces
 * the same rule on the page's API.
 */
export default function MenuGuard({ children }: { children: ReactNode }) {
  const { menus } = useLayout();
  const { pathname } = useLocation();
  if (!menus) return <p className="muted">Loading…</p>;
  if (findTrail(menus, pathname).length === 0) return <MenuPage />;
  return <>{children}</>;
}
