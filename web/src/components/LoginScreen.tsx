import { useRef, useState } from "react";
import type { AuthStatus } from "../hooks/useAuth";

export interface LoginScreenProps {
  sessionName: string;
  sessionTitle: string;
  status: AuthStatus;
  error: string | null;
  /** Role this login will grant (SPEC.md §9: viewer|operator) — known ahead
   * of a successful sign-in because it's carried in the session's grant
   * config, not decided by the credentials themselves. */
  role: "viewer" | "operator";
  onSubmit: (username: string, password: string) => void;
}

/** Login screen (SPEC.md §12 M7), matching MOCKUP.html's `.loginscreen`
 * exactly: full-bleed overlay, wordmark, session-target line, credentials
 * form, inline error, disabled "Signing in…" state, and a footer stating
 * the session mechanics. */
export function LoginScreen({ sessionName, sessionTitle, status, error, role, onSubmit }: LoginScreenProps) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const passwordRef = useRef<HTMLInputElement>(null);
  const signingIn = status === "signing-in";

  const handleSubmit = (ev: React.FormEvent) => {
    ev.preventDefault();
    if (signingIn) return;
    onSubmit(username, password);
    setPassword("");
    passwordRef.current?.focus();
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Sign in to ditty"
      className="absolute inset-0 z-40 grid place-items-center bg-ink-deep p-6"
    >
      <div className="w-full max-w-[336px]">
        <p className="mb-6 flex items-baseline gap-2 font-sans text-[19px] font-bold tracking-tight text-[#F2F5F9]">
          <Logomark />
          ditty
        </p>
        <p className="mb-5 font-mono text-xs text-muted-bright">
          Signing in to <b className="font-sans font-semibold text-[#E4E7EC]">{sessionName}</b> · {sessionTitle}
        </p>
        <form onSubmit={handleSubmit} noValidate>
          <div className="mb-3.5">
            <label htmlFor="lf-user" className="mb-1.5 block text-xs text-muted-bright">
              Username
            </label>
            <input
              id="lf-user"
              type="text"
              autoComplete="username"
              autoFocus
              required
              value={username}
              onChange={(ev) => setUsername(ev.target.value)}
              className={`w-full rounded-md border bg-ink-soft px-2.5 py-2 font-mono text-[13px] text-[#E4E7EC] focus-visible:border-pine focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-pine ${
                error ? "border-rust/60" : "border-line"
              }`}
            />
          </div>
          <div className="mb-3.5">
            <label htmlFor="lf-pass" className="mb-1.5 block text-xs text-muted-bright">
              Password
            </label>
            <input
              id="lf-pass"
              ref={passwordRef}
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(ev) => setPassword(ev.target.value)}
              className={`w-full rounded-md border bg-ink-soft px-2.5 py-2 font-mono text-[13px] text-[#E4E7EC] focus-visible:border-pine focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-pine ${
                error ? "border-rust/60" : "border-line"
              }`}
            />
          </div>
          {error && (
            <p role="alert" className="mb-3.5 -mt-1 border-l-2 border-rust pl-2.5 text-xs leading-relaxed text-[#E08070]">
              {error}
            </p>
          )}
          <button
            type="submit"
            disabled={signingIn}
            className="w-full rounded-md bg-pine py-2 text-center font-sans text-sm font-medium text-[#0e1116] disabled:cursor-default disabled:opacity-60"
          >
            {signingIn ? "Signing in…" : "Sign in"}
          </button>
        </form>
        <p className="mt-5 font-mono text-[11.5px] text-muted">
          JWT session · 15 minute access token · role: {role}
        </p>
      </div>
    </div>
  );
}

function Logomark() {
  return (
    <svg className="h-[18px] w-[18px]" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <rect x="1.4" y="2.9" width="17.2" height="12.4" rx="2.1" stroke="currentColor" strokeWidth="1.4" />
      <path
        d="M4.3 6.7l2.6 2.3-2.6 2.3"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <rect x="8.3" y="10.3" width="3.3" height="1.5" rx="0.3" fill="var(--ditty-chrome-accent)" />
      <path d="M6.5 17.4h7" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  );
}
