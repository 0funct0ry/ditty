import { useCallback, useEffect, useRef, useState } from "react";
import type { AuthClient } from "../protocol/auth";

export type AuthStatus = "gate" | "signing-in" | "authed";

export interface UseAuthResult {
  status: AuthStatus;
  role: "viewer" | "operator" | null;
  error: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

/**
 * Thin React wrapper around an AuthClient (SPEC.md §12 M7). Depends only on
 * the AuthClient interface, never on the fixture implementation, so M12 can
 * swap the concrete client without touching this hook or its callers.
 *
 * required and alreadyAuthenticated both arrive asynchronously (App.tsx
 * probes the real backend — there is no synchronous Hello field for
 * either), so an effect keeps status in sync with them rather than only
 * reading them once at the initial render.
 *
 * loggedInRef tracks whether the current "authed" status reflects a real,
 * verified session — either a successful login() call, or
 * alreadyAuthenticated being true on arrival (a page refresh after signing
 * in) — as opposed to the hook's initial optimistic "authed" default
 * (required starts false until the probe resolves). Without this
 * distinction, required's first, optimistic false render would set status
 * to "authed" once, and a later required=true render would then see
 * status==="authed" and never show the gate — the same bug that made the
 * sign-out button vanish on refresh once alreadyAuthenticated existed:
 * required and alreadyAuthenticated resolve together, so both must be
 * read together, not folded into one boolean.
 */
export function useAuth(client: AuthClient, required: boolean, alreadyAuthenticated: boolean): UseAuthResult {
  const [status, setStatus] = useState<AuthStatus>(required ? "gate" : "authed");
  const [role, setRole] = useState<"viewer" | "operator" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const loggedInRef = useRef(false);

  useEffect(() => {
    if (!required) {
      setStatus("authed");
      return;
    }
    if (alreadyAuthenticated) {
      loggedInRef.current = true;
      setStatus("authed");
      return;
    }
    if (!loggedInRef.current) {
      setStatus("gate");
    }
  }, [required, alreadyAuthenticated]);

  const login = useCallback(
    async (username: string, password: string) => {
      setStatus("signing-in");
      setError(null);
      const result = await client.login(username, password);
      if (result.ok) {
        loggedInRef.current = true;
        setRole(result.role);
        setStatus("authed");
      } else {
        setError(result.reason);
        setStatus("gate");
      }
    },
    [client],
  );

  const logout = useCallback(() => {
    void client.logout();
    loggedInRef.current = false;
    setRole(null);
    setError(null);
    setStatus("gate");
  }, [client]);

  return { status, role, error, login, logout };
}
