// AuthClient (SPEC.md §12 M7): the boundary between the login screen and
// whatever answers its credentials. M7 ships a dev fixture responder
// (fixtureAuthClient.ts); M12 implements this same interface against the
// real SQLite/JWT backend (SPEC.md §6.2, §9) without touching a component.

export type LoginResult = { ok: true; role: "viewer" | "operator" } | { ok: false; reason: string };

export interface AuthClient {
  login(username: string, password: string): Promise<LoginResult>;
  logout(): Promise<void>;
}
