import type { AuthClient, LoginResult } from "./auth";

const SIGN_IN_DELAY_MS = 550;
const LOCKOUT_THRESHOLD = 5;
const LOCKOUT_WINDOW_MS = 60_000;
const LOCKOUT_DURATION_MS = 30_000;

/**
 * Dev-only AuthClient (SPEC.md §12 M7): the real backend doesn't exist
 * until M12, so this fixture responder stands in for it. Any non-empty
 * credentials succeed after a simulated delay; failure and lockout are
 * reachable on demand via query params so the login screen's error states
 * can be exercised without a real throttle behind them (that's M12's job,
 * per §9's 5-per-minute rule).
 */
export class FixtureAuthClient implements AuthClient {
  private failures: number[] = [];
  private lockedUntil = 0;

  async login(username: string, password: string): Promise<LoginResult> {
    await delay(SIGN_IN_DELAY_MS);

    const now = Date.now();
    if (now < this.lockedUntil) {
      return { ok: false, reason: lockoutMessage(this.lockedUntil - now) };
    }

    const params = new URLSearchParams(window.location.search);
    const forceFail = params.get("authfail") === "1";

    if (forceFail || !username.trim() || !password.trim()) {
      this.failures = this.failures.filter((t) => now - t < LOCKOUT_WINDOW_MS);
      this.failures.push(now);
      if (this.failures.length >= LOCKOUT_THRESHOLD) {
        this.lockedUntil = now + LOCKOUT_DURATION_MS;
        this.failures = [];
        return { ok: false, reason: lockoutMessage(LOCKOUT_DURATION_MS) };
      }
      return { ok: false, reason: "Incorrect username or password." };
    }

    this.failures = [];
    const role = params.get("role") === "viewer" ? "viewer" : "operator";
    return { ok: true, role };
  }

  async logout(): Promise<void> {
    this.failures = [];
    this.lockedUntil = 0;
  }
}

function lockoutMessage(remainingMs: number): string {
  const seconds = Math.max(1, Math.ceil(remainingMs / 1000));
  return `Too many attempts. Try again in ${seconds}s.`;
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Whether the fixture gates the session behind a login screen at all
 * (SPEC.md §6.2 --auth-db). No real Hello field exists for this until M12
 * wires --auth-db through the protocol, so M7 reads it from the URL. */
export function fixtureAuthRequired(): boolean {
  return new URLSearchParams(window.location.search).get("auth") === "1";
}

/** Role this fixture login will grant on success (SPEC.md §9). */
export function fixtureExpectedRole(): "viewer" | "operator" {
  return new URLSearchParams(window.location.search).get("role") === "viewer" ? "viewer" : "operator";
}
