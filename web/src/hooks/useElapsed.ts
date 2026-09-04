import { useEffect, useState } from "react";

/** Formats a duration in seconds as SPEC.md §10's "4m12s" style, dropping
 * leading zero units (no "0h4m12s"). */
export function formatDurationWords(totalSeconds: number): string {
  const total = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) return `${h}h${m}m${s}s`;
  if (m > 0) return `${m}m${s}s`;
  return `${s}s`;
}

/** Formats a duration in seconds as a clock (mm:ss, or h:mm:ss past an hour)
 * for the status bar's live elapsed-time counter. */
export function formatClock(totalSeconds: number): string {
  const total = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

/** Ticks once a second and returns the clock-formatted elapsed time since
 * startedAt (an RFC3339 timestamp from Hello.session.startedAt). */
export function useElapsedClock(startedAt: string | undefined): string {
  const [, tick] = useState(0);

  useEffect(() => {
    if (!startedAt) return;
    const id = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, [startedAt]);

  if (!startedAt) return "--:--";
  const start = new Date(startedAt).getTime();
  if (Number.isNaN(start)) return "--:--";
  return formatClock((Date.now() - start) / 1000);
}
