import { useEffect, useRef, useState } from "react";
import type { Roster } from "../protocol/types";

const NOTE_DURATION_MS = 4000;

/**
 * Diffs successive Roster broadcasts into a single transient note — "Client
 * 3 joined" / "Client 2 left" — that fades out on its own (SPEC.md §10.1's
 * "transient one-line note in the status bar", no toast stack). The first
 * Roster this Client itself receives on join is not announced.
 */
export function useRosterNote(roster: Roster | null): string | null {
  const [note, setNote] = useState<string | null>(null);
  const knownLabels = useRef<Map<string, string> | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!roster) return;
    const prev = knownLabels.current;
    const next = new Map(roster.clients.map((c) => [c.id, c.label] as const));

    if (prev) {
      const joined = roster.clients.find((c) => !prev.has(c.id));
      const left = [...prev].find(([id]) => !next.has(id));
      const text = joined ? `${joined.label} joined` : left ? `${left[1]} left` : null;
      if (text) {
        setNote(text);
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = setTimeout(() => setNote(null), NOTE_DURATION_MS);
      }
    }

    knownLabels.current = next;
  }, [roster]);

  useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current);
  }, []);

  return note;
}
