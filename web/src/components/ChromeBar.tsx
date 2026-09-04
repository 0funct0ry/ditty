import type { ConnectionState } from "../protocol/connection";

export interface ChromeBarProps {
  sessionName: string;
  title: string;
  connectionState: ConnectionState;
  writable: boolean;
  clientCount: number;
  onSettingsClick: () => void;
  onShareClick: () => void;
}

const DOT_CLASS: Record<ConnectionState, string> = {
  connecting: "bg-saffron",
  open: "bg-saffron",
  live: "bg-pine animate-dot-pulse",
  reconnecting: "bg-saffron",
  closed: "bg-muted",
  rejected: "bg-rust",
};

/** Chrome bar: state dot, session name, title, access badge, Client count,
 * settings and share buttons (SPEC.md §10). Collapses to icon-only under
 * 640px via the sm: breakpoint. */
export function ChromeBar({
  sessionName,
  title,
  connectionState,
  writable,
  clientCount,
  onSettingsClick,
  onShareClick,
}: ChromeBarProps) {
  return (
    <div className="flex h-chrome shrink-0 items-center gap-3 border-b border-line bg-ink px-3 font-sans text-sm text-[#E4E7EC]">
      <span
        aria-hidden="true"
        className={`h-2 w-2 shrink-0 rounded-full ${DOT_CLASS[connectionState]}`}
      />
      <span className="shrink-0 font-semibold">{sessionName}</span>
      <span className="hidden truncate font-mono text-xs text-muted-bright sm:inline">{title}</span>
      <span className="flex-1" />
      <span
        className={`shrink-0 rounded border px-2 py-0.5 text-xs ${
          writable ? "border-saffron text-saffron" : "border-line text-muted-bright"
        }`}
      >
        {writable ? "you can type" : "read-only"}
      </span>
      <span className="hidden shrink-0 font-mono text-xs text-muted-bright sm:inline">
        {clientCount} watching
      </span>
      <button
        type="button"
        aria-label="Settings"
        onClick={onSettingsClick}
        className="shrink-0 rounded p-1.5 text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        <GearIcon />
      </button>
      <button
        type="button"
        aria-label="Share"
        onClick={onShareClick}
        className="shrink-0 rounded p-1.5 text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        <ShareIcon />
      </button>
    </div>
  );
}

function GearIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  );
}

function ShareIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="18" cy="5" r="3" />
      <circle cx="6" cy="12" r="3" />
      <circle cx="18" cy="19" r="3" />
      <path d="M8.6 13.5 15.4 17.5M15.4 6.5 8.6 10.5" />
    </svg>
  );
}
