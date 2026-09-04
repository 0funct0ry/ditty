import type { Profile } from "./types";

export const PROFILE_STORAGE_KEY = "ditty.profile.v1";

/** Merges the server-seeded Profile from Hello with a client override
 * (SPEC.md §7 — server is a seed, not a lock: an override field wins where
 * present, otherwise the server's value stands). Pure, no DOM dependency,
 * so it's unit-testable independent of localStorage or React. */
export function mergeProfile(server: Profile, override: Partial<Profile> | null): Profile {
  return override ? { ...server, ...override } : server;
}

export function readStoredProfile(storage: Pick<Storage, "getItem">): Partial<Profile> | null {
  try {
    const raw = storage.getItem(PROFILE_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Partial<Profile>) : null;
  } catch {
    return null;
  }
}

export function writeStoredProfile(storage: Pick<Storage, "setItem">, profile: Profile): void {
  try {
    storage.setItem(PROFILE_STORAGE_KEY, JSON.stringify(profile));
  } catch {
    // localStorage unavailable (private mode, disabled) — settings simply
    // don't persist across reloads; nothing else in the app depends on it.
  }
}

export function clearStoredProfile(storage: Pick<Storage, "removeItem">): void {
  try {
    storage.removeItem(PROFILE_STORAGE_KEY);
  } catch {
    // see writeStoredProfile
  }
}
