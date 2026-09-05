import { useEffect, useRef, useState } from "react";
import { ChromeBar } from "./components/ChromeBar";
import { StatusBar } from "./components/StatusBar";
import { Terminal } from "./components/Terminal";
import { ReadOnlyToast } from "./components/ReadOnlyToast";
import { SettingsDrawer } from "./components/SettingsDrawer";
import { ShareSheet } from "./components/ShareSheet";
import { LoginScreen } from "./components/LoginScreen";
import { ClosedOverlay, ConnectingOverlay, ReconnectOverlay, RejectedOverlay } from "./components/Overlays";
import { useDittyConnection } from "./hooks/useDittyConnection";
import { useRosterNote } from "./hooks/useRosterNote";
import { useProfile } from "./hooks/useProfile";
import { useAuth } from "./hooks/useAuth";
import { resolveChromeTint, resolveTheme } from "./protocol/themes";
import { FixtureAuthClient, fixtureAuthRequired, fixtureExpectedRole } from "./protocol/fixtureAuthClient";

// Constructed once at the composition root (SPEC.md §12 M7) — App and its
// children depend on the AuthClient interface only, so M12 can swap this
// for a real HTTP-backed client without touching a component.
const authClient = new FixtureAuthClient();
const authRequired = fixtureAuthRequired();

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
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  // Hello.session.cols/rows is the size at attach time only (0x0 for a
  // dynamically-sized Session, SPEC.md §5.4) — there is no protocol frame
  // that echoes a live resize back, so the status bar tracks the terminal's
  // actual current size from Terminal's own onResize calls instead.
  const [liveDims, setLiveDims] = useState<{ cols: number; rows: number } | null>(null);

  const { state, hello, roster, exit, writable, sizing, rejectReason, send, resize, retryNow } = useDittyConnection(
    wsURL(),
    { onOutput: (data) => writeRef.current?.(data) },
  );

  const handleResize = (cols: number, rows: number) => {
    setLiveDims({ cols, rows });
    resize(cols, rows);
  };

  const rosterNote = useRosterNote(roster);
  const { profile, setProfile, resetToServerDefaults, locked } = useProfile(hello);
  const auth = useAuth(authClient, authRequired);
  const chromeTint = resolveChromeTint(profile);
  const terminalTheme = resolveTheme(profile);

  useEffect(() => {
    if (!readOnlyToast) return;
    const timer = setTimeout(() => setReadOnlyToast(false), 2500);
    return () => clearTimeout(timer);
  }, [readOnlyToast]);

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

  const sessionName = hello?.session.name ?? "session";
  const sessionTitle = hello?.session.title ?? sessionName;
  const clientCount = roster?.count ?? (hello ? 1 : 0);

  return (
    <div
      className="relative flex h-full flex-col"
      style={
        {
          "--ditty-chrome-bg": chromeTint.bg,
          "--ditty-chrome-border": chromeTint.border,
          "--ditty-chrome-accent": chromeTint.accent,
          "--ditty-terminal-bg": terminalTheme.background,
        } as React.CSSProperties
      }
    >
      <div aria-live="polite" className="sr-only">
        {STATE_ANNOUNCEMENT[state] ?? ""}
        {auth.status === "gate" && authRequired ? "Sign in required" : ""}
      </div>
      {authRequired && auth.status !== "authed" && (
        <LoginScreen
          sessionName={sessionName}
          sessionTitle={sessionTitle}
          status={auth.status}
          error={auth.error}
          role={fixtureExpectedRole()}
          onSubmit={(username, password) => void auth.login(username, password)}
        />
      )}
      <ChromeBar
        sessionName={sessionName}
        title={sessionTitle}
        connectionState={state}
        writable={writable}
        clientCount={clientCount}
        settingsHidden={locked}
        onSettingsClick={() => setSettingsOpen((v) => !v)}
        onShareClick={() => setShareOpen((v) => !v)}
        onSignOutClick={authRequired && auth.status === "authed" ? auth.logout : undefined}
      />
      <div className="relative min-h-0 flex-1 overflow-hidden">
        <Terminal
          profile={profile}
          writable={writable}
          sizing={sizing}
          writeRef={writeRef}
          onInput={send}
          onResize={handleResize}
          onReadOnlyKeystroke={() => setReadOnlyToast(true)}
        />
        {readOnlyToast && <ReadOnlyToast />}
        {!locked && (
          <SettingsDrawer
            open={settingsOpen}
            profile={profile}
            onChange={setProfile}
            onReset={resetToServerDefaults}
            onClose={() => setSettingsOpen(false)}
          />
        )}
        <ShareSheet open={shareOpen} sessionName={sessionName} writable={writable} onClose={() => setShareOpen(false)} />
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
        cols={liveDims?.cols ?? hello?.session.cols ?? 0}
        rows={liveDims?.rows ?? hello?.session.rows ?? 0}
        sizing={sizing}
        startedAt={hello?.session.startedAt}
        rosterNote={rosterNote}
      />
    </div>
  );
}
