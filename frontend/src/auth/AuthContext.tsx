import { createContext, ReactNode, useCallback, useContext, useEffect, useRef, useState } from "react";
import { api, TokenResponse } from "../api";

/**
 * Session handling
 * ----------------
 * The backend issues a JWT that expires after JWT_TTL (15 min by default).
 * The inactivity window equals that TTL:
 *   - Any user activity (mouse, keyboard, scroll, touch) updates `lastActivity`.
 *   - While the user stays active the token is refreshed (sliding expiration),
 *     so its expiry always sits ~TTL after the latest activity.
 *   - If no activity happens for TTL, the client logs out, and the token has
 *     expired on the server too, so the rule is enforced on both sides.
 */

const SESSION_KEY = "auth.session";
const ACTIVITY_KEY = "auth.lastActivity";
const CHECK_INTERVAL_MS = 5_000;
const REFRESH_AFTER_ACTIVITY_MS = 30_000; // refresh at most this often while active
const ACTIVITY_EVENTS = ["mousemove", "mousedown", "keydown", "scroll", "touchstart", "wheel"] as const;

interface Session {
  token: string;
  issuedAt: number; // ms epoch, when the client received the token
  expiresAt: number; // ms epoch
}

interface AuthContextValue {
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: (reason?: string) => void;
  logoutReason: string | null;
  clearLogoutReason: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function toSession(res: TokenResponse): Session {
  return { token: res.token, issuedAt: Date.now(), expiresAt: new Date(res.expiresAt).getTime() };
}

function safeGet(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function safeSet(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    /* storage unavailable: session lives in memory only */
  }
}

function loadSession(): Session | null {
  const raw = safeGet(SESSION_KEY);
  if (!raw) return null;
  try {
    const s = JSON.parse(raw) as Session;
    const lastActivity = Number(safeGet(ACTIVITY_KEY) ?? s.issuedAt);
    const ttl = s.expiresAt - s.issuedAt;
    if (Date.now() >= s.expiresAt || Date.now() - lastActivity >= ttl) return null;
    return s;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(() => {
    const s = loadSession();
    if (!s) safeSet(SESSION_KEY, null);
    return s;
  });
  const [logoutReason, setLogoutReason] = useState<string | null>(null);
  const lastActivity = useRef<number>(Number(safeGet(ACTIVITY_KEY)) || Date.now());
  const refreshing = useRef(false);

  const saveSession = useCallback((s: Session | null) => {
    safeSet(SESSION_KEY, s ? JSON.stringify(s) : null);
    setSession(s);
  }, []);

  const logout = useCallback(
    (reason?: string) => {
      saveSession(null);
      setLogoutReason(reason ?? null);
    },
    [saveSession],
  );

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.login(email, password);
      lastActivity.current = Date.now();
      safeSet(ACTIVITY_KEY, String(lastActivity.current));
      setLogoutReason(null);
      saveSession(toSession(res));
    },
    [saveSession],
  );

  // Track user activity.
  useEffect(() => {
    if (!session) return;
    let lastWrite = 0;
    const onActivity = () => {
      const now = Date.now();
      lastActivity.current = now;
      if (now - lastWrite > 1_000) {
        lastWrite = now;
        safeSet(ACTIVITY_KEY, String(now)); // shared with other tabs
      }
    };
    ACTIVITY_EVENTS.forEach((e) => window.addEventListener(e, onActivity, { passive: true }));
    return () => ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, onActivity));
  }, [session]);

  // Periodically enforce the inactivity timeout and slide the token while active.
  useEffect(() => {
    if (!session) return;
    const ttl = session.expiresAt - session.issuedAt;

    const tick = async () => {
      const now = Date.now();
      // Pick up activity from other tabs.
      const shared = Number(safeGet(ACTIVITY_KEY));
      if (shared > lastActivity.current) lastActivity.current = shared;

      if (now - lastActivity.current >= ttl || now >= session.expiresAt) {
        const minutes = Math.max(1, Math.round(ttl / 60_000));
        logout(`You were logged out after ${minutes} minute${minutes === 1 ? "" : "s"} of inactivity.`);
        return;
      }
      if (!refreshing.current && lastActivity.current - session.issuedAt > REFRESH_AFTER_ACTIVITY_MS) {
        refreshing.current = true;
        try {
          saveSession(toSession(await api.refresh(session.token)));
        } catch {
          logout("Your session has expired. Please log in again.");
        } finally {
          refreshing.current = false;
        }
      }
    };

    const id = window.setInterval(tick, CHECK_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [session, logout, saveSession]);

  // Keep tabs in sync: logging in/out in one tab applies to the others.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key !== SESSION_KEY) return;
      setSession(e.newValue ? (JSON.parse(e.newValue) as Session) : null);
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        token: session?.token ?? null,
        login,
        logout,
        logoutReason,
        clearLogoutReason: () => setLogoutReason(null),
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
