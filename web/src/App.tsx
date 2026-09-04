import { useEffect, useRef, useState } from "react";
import { ChromeBar } from "./components/ChromeBar";
import { StatusBar } from "./components/StatusBar";
import { Terminal } from "./components/Terminal";
import { ReadOnlyToast } from "./components/ReadOnlyToast";
import { ClosedOverlay, ConnectingOverlay, ReconnectOverlay, RejectedOverlay } from "./components/Overlays";
import { useDittyConnection } from "./hooks/useDittyConnection";
import { useRosterNote } from "./hooks/useRosterNote";
import { defaultProfile } from "./protocol/defaultProfile";

function wsURL(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws`;
}

/** Parses Hello.policy.reconnectInterval ("1s", "30s") into milliseconds,
 * falling back to 1s if the server ever sends something unparseable. */
function parseReconnectIntervalMs(interval: string | undefined): number {
  const match = interval?.match(/^(\d+(?:\.\d+)?)s$/);
  return match ? Number(match[1]) * 1000 : 1000;
}

const STATE_ANNOUNCEMENT: Record<string, string> = {
  connecting: "Connecting",
  live: "Connected",
  reconnecting: "Connection lost, reconnecting",
  closed: "Session closed",
  rejected: "Connection rejected",
};

export default function App() {
  const writeRef = useRef<((data: Uint8Array) => void) | null>(null);
  const [readOnlyToast, setReadOnlyToast] = useState(false);
  const [shareCopied, setShareCopied] = useState(false);

  const { state, hello, roster, exit, writable, sizing, rejectReason, send, resize, retryNow } = useDittyConnection(
    wsURL(),
    { onOutput: (data) => writeRef.current?.(data) },
  );

  const rosterNote = useRosterNote(roster);

  useEffect(() => {
    if (!readOnlyToast) return;
    const timer = setTimeout(() => setReadOnlyToast(false), 2500);
    return () => clearTimeout(timer);
  }, [readOnlyToast]);

  useEffect(() => {
    if (!shareCopied) return;
    const timer = setTimeout(() => setShareCopied(false), 2500);
    return () => clearTimeout(timer);
  }, [shareCopied]);

  useEffect(() => {
    if (!hello?.policy.unloadWarning) return;
    const handler = (ev: BeforeUnloadEvent) => {
      if (state !== "live") return;
      ev.preventDefault();
      ev.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [hello?.policy.unloadWarning, state]);

  useEffect(() => {
    document.title = hello?.session.title || hello?.session.name || "ditty";
  }, [hello?.session.title, hello?.session.name]);

  const profile = hello?.profile ?? defaultProfile;
  const sessionName = hello?.session.name ?? "session";
  const sessionTitle = hello?.session.title ?? sessionName;
  const clientCount = roster?.count ?? (hello ? 1 : 0);

  return (
    <div className="flex h-full flex-col">
      <div aria-live="polite" className="sr-only">
        {STATE_ANNOUNCEMENT[state] ?? ""}
      </div>
      <ChromeBar
        sessionName={sessionName}
        title={sessionTitle}
        connectionState={state}
        writable={writable}
        clientCount={clientCount}
        onSettingsClick={() => {
          /* Settings drawer contents arrive in M6; the button is reachable
           * and keyboard-focusable now so the chrome layout doesn't shift. */
        }}
        onShareClick={() => {
          void navigator.clipboard.writeText(window.location.href).then(() => setShareCopied(true));
        }}
      />
      <div className="relative min-h-0 flex-1">
        <Terminal
          profile={profile}
          writable={writable}
          sizing={sizing}
          writeRef={writeRef}
          onInput={send}
          onResize={resize}
          onReadOnlyKeystroke={() => setReadOnlyToast(true)}
        />
        {readOnlyToast && <ReadOnlyToast />}
        {shareCopied && (
          <div className="absolute right-3 top-3 rounded-md border border-line bg-ink-soft px-3 py-2 font-sans text-sm text-[#E4E7EC]">
            Link copied
          </div>
        )}
        {state === "connecting" && <ConnectingOverlay sessionName={sessionName} />}
        {state === "reconnecting" && (
          <ReconnectOverlay
            reconnectIntervalMs={parseReconnectIntervalMs(hello?.policy.reconnectInterval)}
            onRetryNow={retryNow}
          />
        )}
        {state === "rejected" && <RejectedOverlay reason={rejectReason} onRetryNow={retryNow} />}
        {state === "closed" && exit && (
          <ClosedOverlay
            sessionTitle={sessionTitle}
            exit={exit}
            startedAt={hello?.session.startedAt}
            clientCount={clientCount}
          />
        )}
      </div>
      <StatusBar
        cols={hello?.session.cols ?? 0}
        rows={hello?.session.rows ?? 0}
        sizing={sizing}
        startedAt={hello?.session.startedAt}
        rosterNote={rosterNote}
      />
    </div>
  );
}
