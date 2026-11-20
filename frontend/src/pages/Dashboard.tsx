import { useEffect, useState } from "react";
import { Building2 } from "lucide-react";
import { api, ApiError, MyBranch } from "../api";
import { useAuth } from "../auth/AuthContext";
import { useLayout } from "../layout/AppLayout";

/** Home: the app title, a welcome line and the branches the user is mapped to. */
export default function Dashboard() {
  const { me } = useLayout();
  const { token, logout } = useAuth();
  const [branches, setBranches] = useState<MyBranch[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .myBranches(token)
      .then((res) => setBranches(res.branches))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load your branches");
      });
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <section className="panel home">
      <h1 className="home-title">Rekam Medis Elektronik (RME)</h1>
      {me ? (
        <p className="home-welcome" data-testid="welcome">
          {me.message}
        </p>
      ) : (
        <p className="muted">Loading…</p>
      )}

      <h2 className="home-sub">Cabang Anda{branches ? ` (${branches.length})` : ""}</h2>
      {error ? (
        <p className="alert error" role="alert">
          {error}
        </p>
      ) : !branches ? (
        <p className="muted">Loading…</p>
      ) : branches.length === 0 ? (
        <p className="alert info" role="status">
          Anda belum terdaftar di cabang mana pun. Minta admin menambahkan cabang di Management › User.
        </p>
      ) : (
        <ul className="home-branches" data-testid="my-branches">
          {branches.map((b) => (
            <li key={b.kode}>
              <Building2 aria-hidden="true" />
              <span className="home-branch-name">{b.nama || b.kode}</span>
              <span className="home-branch-code">{b.kode}</span>
            </li>
          ))}
        </ul>
      )}

      <p className="muted home-note">You will be logged out automatically after 15 minutes of inactivity.</p>
    </section>
  );
}
