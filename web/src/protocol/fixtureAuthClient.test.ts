import { afterEach, describe, expect, it, vi } from "vitest";
import { FixtureAuthClient, fixtureAuthRequired, fixtureExpectedRole } from "./fixtureAuthClient";

function setSearch(search: string) {
  window.history.replaceState(null, "", `/${search}`);
}

afterEach(() => {
  setSearch("");
  vi.useRealTimers();
});

describe("fixtureAuthRequired / fixtureExpectedRole", () => {
  it("reads auth and role off the query string", () => {
    setSearch("?auth=1&role=viewer");
    expect(fixtureAuthRequired()).toBe(true);
    expect(fixtureExpectedRole()).toBe("viewer");
  });

  it("defaults to no auth and operator", () => {
    setSearch("");
    expect(fixtureAuthRequired()).toBe(false);
    expect(fixtureExpectedRole()).toBe("operator");
  });
});

describe("FixtureAuthClient", () => {
  it("accepts any non-empty credentials", async () => {
    vi.useFakeTimers();
    const client = new FixtureAuthClient();
    const promise = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toEqual({ ok: true, role: "operator" });
  });

  it("grants the role requested via ?role=viewer", async () => {
    vi.useFakeTimers();
    setSearch("?role=viewer");
    const client = new FixtureAuthClient();
    const promise = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toEqual({ ok: true, role: "viewer" });
  });

  it("rejects empty credentials with the mockup's error copy", async () => {
    vi.useFakeTimers();
    const client = new FixtureAuthClient();
    const promise = client.login("", "");
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toEqual({ ok: false, reason: "Incorrect username or password." });
  });

  it("forces failure via ?authfail=1 even with real-looking credentials", async () => {
    vi.useFakeTimers();
    setSearch("?authfail=1");
    const client = new FixtureAuthClient();
    const promise = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toEqual({ ok: false, reason: "Incorrect username or password." });
  });

  it("locks out after 5 failures within a minute, then unlocks", async () => {
    vi.useFakeTimers();
    setSearch("?authfail=1");
    const client = new FixtureAuthClient();

    for (let i = 0; i < 4; i++) {
      const p = client.login("anirban", "wrong");
      await vi.runAllTimersAsync();
      await expect(p).resolves.toEqual({ ok: false, reason: "Incorrect username or password." });
    }

    const lockoutAttempt = client.login("anirban", "wrong");
    await vi.runAllTimersAsync();
    const lockoutResult = await lockoutAttempt;
    expect(lockoutResult.ok).toBe(false);
    if (!lockoutResult.ok) expect(lockoutResult.reason).toMatch(/Too many attempts\. Try again in \d+s\./);

    // Still locked immediately after, even with correct credentials.
    setSearch("");
    const stillLocked = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    const stillLockedResult = await stillLocked;
    expect(stillLockedResult.ok).toBe(false);

    // Advance past the 30s lockout window; correct credentials now succeed.
    vi.advanceTimersByTime(31_000);
    const afterUnlock = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    await expect(afterUnlock).resolves.toEqual({ ok: true, role: "operator" });
  });

  it("logout clears failure count and lockout state", async () => {
    vi.useFakeTimers();
    setSearch("?authfail=1");
    const client = new FixtureAuthClient();
    for (let i = 0; i < 5; i++) {
      const p = client.login("anirban", "wrong");
      await vi.runAllTimersAsync();
      await p;
    }
    await client.logout();

    setSearch("");
    const afterLogout = client.login("anirban", "hunter22");
    await vi.runAllTimersAsync();
    await expect(afterLogout).resolves.toEqual({ ok: true, role: "operator" });
  });
});
