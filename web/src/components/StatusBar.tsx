import { SUBPROTOCOL } from "../protocol/opcodes";
import { useElapsedClock } from "../hooks/useElapsed";

export interface StatusBarProps {
  cols: number;
  rows: number;
  sizing: boolean;
  startedAt: string | undefined;
  rosterNote: string | null;
}

/** Status bar: dimensions, protocol version, sizing attribution, elapsed
 * time, and a transient roster-change note (SPEC.md §10). */
export function StatusBar({ cols, rows, sizing, startedAt, rosterNote }: StatusBarProps) {
  const elapsed = useElapsedClock(startedAt);

  return (
    <div className="flex h-status shrink-0 items-center gap-3 border-t border-line bg-ink px-3 font-mono text-xs text-muted-bright">
      <span className="border-r border-line pr-3">
        {cols}×{rows}
      </span>
      <span className="border-r border-line pr-3">{SUBPROTOCOL}</span>
      <span className="border-r border-line pr-3">{sizing ? "sized by you" : "sized by another client"}</span>
      <span>{elapsed}</span>
      {rosterNote && (
        <span key={rosterNote} className="animate-fade-in-out text-pine">
          {rosterNote}
        </span>
      )}
    </div>
  );
}
