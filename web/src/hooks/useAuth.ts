import { useCallback, useState } from "react";
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
 */
export function useAuth(client: AuthClient, required: boolean): UseAuthResult {
  const [status, setStatus] = useState<AuthStatus>(required ? "gate" : "authed");
  const [role, setRole] = useState<"viewer" | "operator" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const login = useCallback(
    async (username: string, password: string) => {
      setStatus("signing-in");
      setError(null);
      const result = await client.login(username, password);
      if (result.ok) {
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
    setRole(null);
    setError(null);
    setStatus("gate");
  }, [client]);

  return { status, role, error, login, logout };
}
