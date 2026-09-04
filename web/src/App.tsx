import { useEffect, useRef, useState } from "react";
import { Terminal } from "./components/Terminal";
import { useDittyConnection } from "./hooks/useDittyConnection";
import { defaultProfile } from "./protocol/defaultProfile";

function wsURL(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws`;
}

export default function App() {
  const writeRef = useRef<((data: Uint8Array) => void) | null>(null);
  const [readOnlyToast, setReadOnlyToast] = useState(false);

  const { state, hello, exit, writable, sizing, send, resize } = useDittyConnection(wsURL(), {
    onOutput: (data) => writeRef.current?.(data),
  });

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

  const profile = hello?.profile ?? defaultProfile;

  return (
    <div className="flex h-full flex-col">
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
        {readOnlyToast && (
          <div className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-md border border-line bg-ink-soft px-3 py-2 font-sans text-sm text-[#E4E7EC]">
            This session is read-only
          </div>
        )}
        {state === "connecting" && (
          <div className="absolute inset-0 grid place-items-center bg-[rgba(14,17,22,0.6)] font-mono text-sm text-muted-bright">
            connecting…
          </div>
        )}
        {state === "reconnecting" && (
          <div className="absolute inset-0 grid place-items-center bg-[rgba(14,17,22,0.72)] font-mono text-sm text-muted-bright">
            reconnecting…
          </div>
        )}
        {state === "rejected" && (
          <div className="absolute inset-0 grid place-items-center bg-ink-deep font-mono text-sm text-rust">
            connection rejected
          </div>
        )}
        {state === "closed" && exit && (
          <div className="absolute inset-0 grid place-items-center bg-[rgba(14,17,22,0.82)] font-mono text-sm text-muted-bright">
            session ended — exit code {exit.code}
            {exit.signal ? ` (signal ${exit.signal})` : ""}
          </div>
        )}
      </div>
    </div>
  );
}
