import { afterEach, describe, expect, it, vi } from "vitest";
import { HttpAuthClient, probeAuthState } from "./httpAuthClient";

function mockFetch(response: { status: number; body?: unknown } | { throws: true }) {
  const impl = "throws" in response
    ? vi.fn().mockRejectedValue(new Error("network down"))
    : vi.fn().mockResolvedValue({
        ok: response.status >= 200 && response.status < 300,
        status: response.status,
        json: async () => response.body ?? {},
      });
  vi.stubGlobal("fetch", impl);
  return impl;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("HttpAuthClient.login", () => {
  it("resolves ok:true with the granted role on a 200", async () => {
    const fetchMock = mockFetch({ status: 200, body: { ok: true, role: "operator" } });
    const client = new HttpAuthClient();
    await expect(client.login("alice", "hunter22ok")).resolves.toEqual({ ok: true, role: "operator" });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/login",
      expect.objectContaining({
        method: "POST",
        credentials: "same-origin",
        body: JSON.stringify({ username: "alice", password: "hunter22ok" }),
      }),
    );
  });

  it("resolves ok:false with the server's reason on a 401", async () => {
    mockFetch({ status: 401, body: { ok: false, reason: "incorrect username or password" } });
    const client = new HttpAuthClient();
    await expect(client.login("alice", "wrong")).resolves.toEqual({
      ok: false,
      reason: "incorrect username or password",
    });
  });

  it("falls back to a generic reason when the server sends no body", async () => {
    mockFetch({ status: 401 });
    const client = new HttpAuthClient();
    await expect(client.login("alice", "wrong")).resolves.toEqual({ ok: false, reason: "Sign in failed." });
  });

  it("never reports ok:true without a valid role, even on a 200", async () => {
    mockFetch({ status: 200, body: { ok: true } });
    const client = new HttpAuthClient();
    const result = await client.login("alice", "hunter22ok");
    expect(result.ok).toBe(false);
  });

  it("reports a network error without throwing", async () => {
    mockFetch({ throws: true });
    const client = new HttpAuthClient();
    await expect(client.login("alice", "hunter22ok")).resolves.toEqual({
      ok: false,
      reason: "Could not reach the server. Check your connection and try again.",
    });
  });
});

describe("HttpAuthClient.logout", () => {
  it("posts to /api/logout and never throws even on network failure", async () => {
    mockFetch({ throws: true });
    const client = new HttpAuthClient();
    await expect(client.logout()).resolves.toBeUndefined();
  });
});

describe("probeAuthState", () => {
  it("is required, not-yet-authenticated when /api/session answers 401", async () => {
    mockFetch({ status: 401 });
    await expect(probeAuthState()).resolves.toEqual({ required: true, authenticated: false });
  });

  it("is not required when /api/session answers 200 with authRequired:false (no JWT configured)", async () => {
    mockFetch({ status: 200, body: { id: "x", authRequired: false } });
    await expect(probeAuthState()).resolves.toEqual({ required: false, authenticated: false });
  });

  // Regression test: a page refresh with an already-valid ditty_access
  // cookie also gets a 200 from /api/session, indistinguishable from "no
  // JWT configured at all" unless the body's authRequired field is read —
  // this is exactly the bug that made the sign-out button vanish on
  // refresh.
  it("is required AND already authenticated when /api/session answers 200 with authRequired:true", async () => {
    mockFetch({ status: 200, body: { id: "x", authRequired: true } });
    await expect(probeAuthState()).resolves.toEqual({ required: true, authenticated: true });
  });

  it("treats a missing authRequired field on a 200 as not required (fails closed toward no gate)", async () => {
    mockFetch({ status: 200, body: { id: "x" } });
    await expect(probeAuthState()).resolves.toEqual({ required: false, authenticated: false });
  });

  it("is not required when the request itself errors", async () => {
    mockFetch({ throws: true });
    await expect(probeAuthState()).resolves.toEqual({ required: false, authenticated: false });
  });
});
