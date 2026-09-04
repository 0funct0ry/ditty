import { useEffect, useState } from "react";

export interface ShareSheetProps {
  open: boolean;
  sessionName: string;
  writable: boolean;
  onClose: () => void;
}

/** Share sheet: current URL, a copy button, and one honest sentence about
 * what the recipient will be able to do (SPEC.md §10.2). */
export function ShareSheet({ open, sessionName, writable, onClose }: ShareSheetProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) return;
    setCopied(false);
    const handler = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  if (!open) return null;

  const url = window.location.href;

  return (
    <div
      role="dialog"
      aria-label="Share this session"
      className="absolute right-3 top-3 z-10 w-[min(380px,calc(100%-24px))] rounded-md border border-[var(--ditty-chrome-border)] bg-ink-soft p-4 font-sans text-sm text-[#E4E7EC] shadow-lg"
    >
      <h3 className="font-semibold">Share {sessionName}</h3>
      <p className="mt-2 text-xs text-muted-bright">
        {writable
          ? "Anyone with this link can watch and type in this terminal."
          : "Anyone with this link can watch the terminal. They cannot type."}
      </p>
      <div className="mt-3 flex items-center gap-2 rounded border border-line bg-ink px-2 py-1.5">
        <code className="flex-1 truncate font-mono text-xs text-muted-bright">{url}</code>
        <button
          type="button"
          onClick={() => void navigator.clipboard.writeText(url).then(() => setCopied(true))}
          className="shrink-0 rounded border border-line px-2 py-1 text-xs hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <p className="mt-3 text-xs text-muted-bright">
        The link stops working when this session ends. To let someone type, restart ditty with{" "}
        <code className="font-mono">-w</code> — and read the security page first.
      </p>
    </div>
  );
}
