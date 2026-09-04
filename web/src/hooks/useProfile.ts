import { useEffect, useMemo, useState } from "react";
import type { Hello, Profile } from "../protocol/types";
import { defaultProfile } from "../protocol/defaultProfile";
import { clearStoredProfile, mergeProfile, readStoredProfile, writeStoredProfile } from "../protocol/profileStorage";

export interface UseProfileResult {
  /** The live, in-effect Profile: server seed merged with any
   * localStorage-persisted overrides (SPEC.md §7 — a seed, not a lock). */
  profile: Profile;
  setProfile: (patch: Partial<Profile>) => void;
  resetToServerDefaults: () => void;
  /** True once a Hello has arrived and profile-lock is on — the settings
   * drawer must not be reachable at all in that case (SPEC.md §10.2). */
  locked: boolean;
}

/**
 * Thin React wrapper around protocol/profileStorage's pure merge/persist
 * logic. The merge itself is unit-tested independent of this hook and of
 * the connection hook (SPEC.md §12 M6).
 */
export function useProfile(hello: Hello | null): UseProfileResult {
  const serverProfile = useMemo<Profile>(() => hello?.profile ?? defaultProfile, [hello?.profile]);
  const locked = hello?.policy.profileLock ?? false;
  const [override, setOverride] = useState<Partial<Profile> | null>(() => readStoredProfile(window.localStorage));

  // Locked means the server values are forced, full stop — a stale
  // localStorage override from before the session was locked must not
  // apply (SPEC.md §10.2: "forces these Profile values").
  const profile = useMemo<Profile>(
    () => (locked ? serverProfile : mergeProfile(serverProfile, override)),
    [serverProfile, override, locked],
  );

  useEffect(() => {
    if (override) writeStoredProfile(window.localStorage, profile);
    // profile is derived from override + serverProfile; only override
    // changes should trigger a write, or every Hello would re-persist.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [override]);

  return {
    profile,
    setProfile: (patch) => setOverride((prev) => ({ ...prev, ...patch })),
    resetToServerDefaults: () => {
      setOverride(null);
      clearStoredProfile(window.localStorage);
    },
    locked,
  };
}
