import { useEffect, useState } from "react";
import type { Exit } from "../protocol/types";
import { formatDurationWords } from "../hooks/useElapsed";

const RECONNECT_TICK_MS = 1000;

export interface ConnectingOverlayProps {
  sessionName: string;
}

/** SPEC.md §10.1 `connecting`: dimmed terminal, centred session name. */
export function ConnectingOverlay({ sessionName }: ConnectingOverlayProps) {
  return (
    <Overlay tone="ink">
      <p className="font-sans text-base text-[#E4E7EC]">Connecting to {sessionName}</p>
      <p className="mt-1 font-mono text-xs text-muted-bright">Waiting for the server to answer.</p>
    </Overlay>
  );
}

export interface ReconnectOverlayProps {
  reconnectIntervalMs: number;
  onRetryNow: () => void;
}

/** SPEC.md §10.1 `reconnecting`: frame frozen underneath, countdown ticking
 * down to the next automatic attempt, Enter (or the button) retries now. */
export function ReconnectOverlay({ reconnectIntervalMs, onRetryNow }: ReconnectOverlayProps) {
  const [remainingMs, setRemainingMs] = useState(reconnectIntervalMs);

  useEffect(() => {
    setRemainingMs(reconnectIntervalMs);
    const id = setInterval(() => {
      setRemainingMs((ms) => Math.max(0, ms - RECONNECT_TICK_MS));
    }, RECONNECT_TICK_MS);
    return () => clearInterval(id);
  }, [reconnectIntervalMs]);

  useEffect(() => {
    const handler = (ev: KeyboardEvent) => {
      if (ev.key === "Enter") onRetryNow();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onRetryNow]);

  const seconds = Math.ceil(remainingMs / 1000);

  return (
    <Overlay tone="ink-deeper">
      <p className="font-sans text-base text-[#E4E7EC]">Connection lost</p>
      <p className="mt-1 font-mono text-xs text-muted-bright">
        Reconnecting in {seconds}s · press Enter to retry now
      </p>
      <button
        type="button"
        onClick={onRetryNow}
        className="mt-4 rounded border border-line px-3 py-1.5 font-sans text-sm text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        Retry now
      </button>
    </Overlay>
  );
}

export interface ClosedOverlayProps {
  sessionTitle: string;
  exit: Exit;
  startedAt: string | undefined;
  clientCount: number;
}

/** SPEC.md §10.1 `closed`: frozen terminal, real exit summary, no
 * auto-reconnect. */
export function ClosedOverlay({ sessionTitle, exit, startedAt, clientCount }: ClosedOverlayProps) {
  const [copied, setCopied] = useState(false);
  const durationSeconds = startedAt ? (Date.now() - new Date(startedAt).getTime()) / 1000 : 0;
  const outcome = exit.signal ? `killed by signal ${exit.signal}` : `exited (code ${exit.code})`;

  return (
    <Overlay tone="ink-deepest">
      <p className="font-sans text-base text-[#E4E7EC]">
        {sessionTitle} {outcome}
      </p>
      <p className="mt-1 font-mono text-xs text-muted-bright">
        after {formatDurationWords(durationSeconds)} · {clientCount} client{clientCount === 1 ? "" : "s"} watching
      </p>
      <button
        type="button"
        onClick={() => {
          const transcript = document.querySelector(".xterm-screen")?.textContent ?? "";
          void navigator.clipboard.writeText(transcript).then(() => setCopied(true));
        }}
        className="mt-4 rounded border border-line px-3 py-1.5 font-sans text-sm text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        {copied ? "Copied" : "Copy transcript"}
      </button>
    </Overlay>
  );
}

const REJECT_REASONS: Record<string, string> = {
  "too many clients": "This session is full.",
  "subprotocol required: ditty.v1": "This client could not speak the session's protocol.",
};

export interface RejectedOverlayProps {
  reason: string | null;
  onRetryNow: () => void;
}

/** SPEC.md §10.1 `rejected`: full-screen, states the reason and what to do. */
export function RejectedOverlay({ reason, onRetryNow }: RejectedOverlayProps) {
  const headline = (reason && REJECT_REASONS[reason]) ?? "This session rejected the connection.";

  return (
    <Overlay tone="ink-deepest">
      <p className="font-sans text-base text-rust">{headline}</p>
      <p className="mt-1 font-mono text-xs text-muted-bright">
        Wait for a client to disconnect, or ask the host to raise the limit.
      </p>
      <button
        type="button"
        onClick={onRetryNow}
        className="mt-4 rounded border border-line px-3 py-1.5 font-sans text-sm text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        Try again
      </button>
    </Overlay>
  );
}

const TONE_CLASS: Record<string, string> = {
  ink: "bg-[rgba(14,17,22,0.6)]",
  "ink-deeper": "bg-[rgba(14,17,22,0.72)]",
  "ink-deepest": "bg-[rgba(14,17,22,0.85)]",
};

function Overlay({ tone, children }: { tone: string; children: React.ReactNode }) {
  return (
    <div
      className={`absolute inset-0 grid place-items-center text-center motion-reduce:transition-none ${TONE_CLASS[tone]}`}
    >
      <div>{children}</div>
    </div>
  );
}
