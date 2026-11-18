import { useLocation } from "react-router-dom";
import { useLayout } from "../layout/AppLayout";
import { findTrail } from "../layout/menuUtils";

/** Fallback for any route a menu item points to that has no page of its own yet. */
export default function MenuPage() {
  const { menus } = useLayout();
  const { pathname } = useLocation();

  if (!menus) return <p className="muted">Loading…</p>;

  const trail = findTrail(menus, pathname);
  const item = trail[trail.length - 1];
  if (!item) {
    return (
      <section className="panel">
        <h1>Page not available</h1>
        <p className="muted">
          <code>{pathname}</code> does not exist, or your role cannot open it.
        </p>
      </section>
    );
  }
  return (
    <section className="panel">
      <h1>{item.title}</h1>
      <p className="muted">This page has not been built yet.</p>
    </section>
  );
}
