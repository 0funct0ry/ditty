import type { AuthClient, LoginResult } from "./auth";

/**
 * Real AuthClient (SPEC.md §12 M12): implements the same interface
 * fixtureAuthClient.ts stood in for during M7, against the real
 * /api/login, /api/refresh, /api/logout routes internal/httpapi serves
 * when --auth-db is configured. No component depends on this file directly
 * — only App.tsx, at the composition root, chooses which AuthClient to
 * construct.
 */
export class HttpAuthClient implements AuthClient {
  async login(username: string, password: string): Promise<LoginResult> {
    let res: Response;
    try {
      res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ username, password }),
      });
    } catch {
      return { ok: false, reason: "Could not reach the server. Check your connection and try again." };
    }

    const body = (await res.json().catch(() => ({}))) as { ok?: boolean; role?: string; reason?: string };
    if (res.ok && body.ok && (body.role === "viewer" || body.role === "operator")) {
      return { ok: true, role: body.role };
    }
    return { ok: false, reason: body.reason || "Sign in failed." };
  }

  async logout(): Promise<void> {
    try {
      await fetch("/api/logout", { method: "POST", credentials: "same-origin" });
    } catch {
      // Best-effort: the cookies are HttpOnly and short-lived either way,
      // and the caller (useAuth.logout) already resets local UI state
      // regardless of whether this network call succeeds.
    }
  }
}

export interface AuthProbeResult {
  /** A JWTGrant is configured for this server at all (--auth-db). */
  required: boolean;
  /** This browser already carries a valid session for it (a page refresh
   * after signing in) — distinct from `required`, since both "no JWT here"
   * and "JWT here and already signed in" answer /api/session with a 200,
   * and only the response body's `authRequired` field tells them apart. */
  authenticated: boolean;
}

/**
 * Probes whether this session requires sign-in, and whether this browser
 * already has a valid one. Used at the composition root instead of a
 * static flag (SPEC.md §12 M7 used a fixture-only ?auth=1 query param; the
 * real Hello has no such field, so this asks the one endpoint that already
 * gates on every Grant type uniformly).
 *
 * /api/session answers 401 when no valid Grant admits the request — that
 * alone means "show the gate". A 200 needs its `authRequired` field read
 * too: without it, "no JWT configured" and "JWT configured, already signed
 * in" are indistinguishable, which is exactly what made the sign-out
 * button vanish on a page refresh after a successful login.
 */
export async function probeAuthState(): Promise<AuthProbeResult> {
  try {
    const res = await fetch("/api/session", { credentials: "same-origin" });
    if (res.status === 401) {
      return { required: true, authenticated: false };
    }
    if (!res.ok) {
      return { required: false, authenticated: false };
    }
    const body = (await res.json().catch(() => ({}))) as { authRequired?: boolean };
    const required = body.authRequired === true;
    return { required, authenticated: required };
  } catch {
    return { required: false, authenticated: false };
  }
}
