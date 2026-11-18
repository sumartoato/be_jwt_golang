import { FormEvent, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { ApiError, FieldErrors } from "../api";
import { useAuth } from "../auth/AuthContext";
import { Field } from "../components";

export default function Login() {
  const { login, logoutReason, clearLogoutReason } = useAuth();
  const navigate = useNavigate();
  const registered = (useLocation().state as { registered?: string } | null)?.registered;

  const [email, setEmail] = useState(registered ?? "");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError("");
    clearLogoutReason();
    const clientErrors: FieldErrors = {};
    if (!email.trim()) clientErrors.email = "Email is required";
    if (!password) clientErrors.password = "Password is required";
    setErrors(clientErrors);
    if (Object.keys(clientErrors).length > 0) return;

    setSubmitting(true);
    try {
      await login(email.trim(), password);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setFormError("Invalid email or password");
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) setErrors(err.fieldErrors);
      else setFormError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="card">
      <h1>Log in</h1>
      {registered && !logoutReason && <p className="alert success">Account created. You can log in now.</p>}
      {logoutReason && <p className="alert info">{logoutReason}</p>}
      <form onSubmit={onSubmit} noValidate>
        <Field label="Email" type="email" value={email} onChange={setEmail} error={errors.email} autoComplete="email" />
        <Field label="Password" type="password" value={password} onChange={setPassword} error={errors.password} autoComplete="current-password" />
        {formError && <p className="alert error">{formError}</p>}
        <button type="submit" disabled={submitting}>
          {submitting ? "Logging in…" : "Log in"}
        </button>
      </form>
      <p className="muted">
        No account yet? <Link to="/register">Register</Link>
      </p>
    </main>
  );
}
