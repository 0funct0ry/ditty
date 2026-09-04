import { describe, expect, it, vi } from "vitest";
import { clearStoredProfile, mergeProfile, readStoredProfile, writeStoredProfile } from "./profileStorage";
import { defaultProfile } from "./defaultProfile";

describe("mergeProfile", () => {
  it("returns the server profile unchanged when there is no override", () => {
    expect(mergeProfile(defaultProfile, null)).toEqual(defaultProfile);
  });

  it("applies override fields on top of the server profile", () => {
    const merged = mergeProfile(defaultProfile, { theme: "nord", fontSize: 18 });
    expect(merged.theme).toBe("nord");
    expect(merged.fontSize).toBe(18);
    expect(merged.fontFamily).toBe(defaultProfile.fontFamily); // untouched field falls through
  });
});

function fakeStorage(initial: Record<string, string> = {}): Storage {
  const store = { ...initial };
  return {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v;
    },
    removeItem: (k: string) => {
      delete store[k];
    },
    clear: () => {
      for (const k of Object.keys(store)) delete store[k];
    },
    key: () => null,
    length: 0,
  };
}

describe("readStoredProfile / writeStoredProfile / clearStoredProfile", () => {
  it("round-trips through storage", () => {
    const storage = fakeStorage();
    writeStoredProfile(storage, { ...defaultProfile, theme: "dracula" });
    expect(readStoredProfile(storage)?.theme).toBe("dracula");
    clearStoredProfile(storage);
    expect(readStoredProfile(storage)).toBeNull();
  });

  it("returns null instead of throwing on malformed JSON", () => {
    const storage = fakeStorage({ "ditty.profile.v1": "{not json" });
    expect(readStoredProfile(storage)).toBeNull();
  });

  it("swallows a storage error instead of throwing", () => {
    const storage: Storage = {
      getItem: vi.fn(() => {
        throw new Error("blocked");
      }),
      setItem: vi.fn(() => {
        throw new Error("blocked");
      }),
      removeItem: vi.fn(() => {
        throw new Error("blocked");
      }),
      clear: vi.fn(),
      key: vi.fn(),
      length: 0,
    };
    expect(readStoredProfile(storage)).toBeNull();
    expect(() => writeStoredProfile(storage, defaultProfile)).not.toThrow();
    expect(() => clearStoredProfile(storage)).not.toThrow();
  });
});
