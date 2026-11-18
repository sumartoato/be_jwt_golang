import { useLayout } from "../layout/AppLayout";

export default function Dashboard() {
  const { me } = useLayout();

  return (
    <section className="panel">
      {me ? <h1 data-testid="welcome">{me.message}</h1> : <p className="muted">Loading…</p>}
      <p className="muted">You will be logged out automatically after 15 minutes of inactivity.</p>
    </section>
  );
}
