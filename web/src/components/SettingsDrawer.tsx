import { useEffect, useRef } from "react";
import type { Profile } from "../protocol/types";
import { THEME_NAMES, THEMES } from "../protocol/themes";
import { Select } from "./ui/Select";

const FONT_FAMILIES = [
  "JetBrains Mono, SF Mono, Menlo, monospace",
  "SF Mono, Menlo, monospace",
  "Menlo, monospace",
  "IBM Plex Mono, monospace",
  "ui-monospace, monospace",
].map((f) => ({ value: f, label: f.split(",")[0] }));

const CURSOR_STYLES = [
  { value: "block", label: "Block" },
  { value: "underline", label: "Underline" },
  { value: "bar", label: "Bar" },
];

const RENDERERS = [
  { value: "webgl", label: "WebGL" },
  { value: "canvas", label: "Canvas" },
];

export interface SettingsDrawerProps {
  open: boolean;
  profile: Profile;
  onChange: (patch: Partial<Profile>) => void;
  onReset: () => void;
  onClose: () => void;
}

/** Settings drawer: 320px, slides over the terminal without reflowing it
 * (SPEC.md §10.2). Theme swatches, font, cursor, renderer, copy-on-select,
 * bell, and a reset back to the server-seeded Profile. */
export function SettingsDrawer({ open, profile, onChange, onReset, onClose }: SettingsDrawerProps) {
  const closeRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (open) closeRef.current?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  return (
    <aside
      aria-label="Terminal settings"
      aria-hidden={!open}
      className={`settings-drawer absolute right-0 top-0 z-10 flex h-full w-full flex-col overflow-y-auto border-l border-[var(--ditty-chrome-border)] bg-ink-soft font-sans text-sm text-[#E4E7EC] sm:w-80 ${
        open ? "translate-x-0" : "pointer-events-none translate-x-full"
      }`}
    >
      <header className="flex shrink-0 items-center justify-between border-b border-[var(--ditty-chrome-border)] px-4 py-3">
        <h3 className="font-semibold">Terminal</h3>
        <button
          ref={closeRef}
          type="button"
          onClick={onClose}
          className="rounded border border-line px-2 py-1 text-xs text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
        >
          Close
        </button>
      </header>

      <div className="flex flex-col gap-5 p-4">
        <Field label="Theme">
          <div className="grid grid-cols-3 gap-2">
            {THEME_NAMES.map((name) => {
              const theme = THEMES[name];
              const pressed = profile.theme === name;
              return (
                <button
                  key={name}
                  type="button"
                  aria-pressed={pressed}
                  onClick={() => onChange({ theme: name })}
                  className={`flex flex-col items-center gap-1.5 rounded border px-2 py-2 text-xs ${
                    pressed ? "border-pine text-[#E4E7EC]" : "border-line text-muted-bright hover:text-[#E4E7EC]"
                  } focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine`}
                >
                  <span className="flex h-4 w-full overflow-hidden rounded-sm" aria-hidden="true">
                    {theme.swatch.map((color, i) => (
                      <span key={i} className="flex-1" style={{ backgroundColor: color }} />
                    ))}
                  </span>
                  {theme.label}
                </button>
              );
            })}
          </div>
        </Field>

        <Field label="Font" htmlFor="ditty-font-family">
          <Select
            id="ditty-font-family"
            value={profile.fontFamily}
            options={FONT_FAMILIES}
            onChange={(v) => onChange({ fontFamily: v })}
          />
        </Field>

        <RowField label="Size">
          <div className="flex items-center gap-2">
            <button
              type="button"
              aria-label="Decrease font size"
              onClick={() => onChange({ fontSize: Math.max(8, profile.fontSize - 1) })}
              className="h-6 w-6 rounded border border-line text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
            >
              −
            </button>
            <span className="w-6 text-center font-mono">{profile.fontSize}</span>
            <button
              type="button"
              aria-label="Increase font size"
              onClick={() => onChange({ fontSize: Math.min(32, profile.fontSize + 1) })}
              className="h-6 w-6 rounded border border-line text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
            >
              +
            </button>
          </div>
        </RowField>

        <Field label="Cursor" htmlFor="ditty-cursor-style">
          <Select
            id="ditty-cursor-style"
            value={profile.cursorStyle}
            options={CURSOR_STYLES}
            onChange={(v) => onChange({ cursorStyle: v as Profile["cursorStyle"] })}
          />
        </Field>

        <RowField label="Cursor blinks">
          <Switch checked={profile.cursorBlink} onChange={(v) => onChange({ cursorBlink: v })} label="Cursor blinks" />
        </RowField>

        <Field label="Renderer" htmlFor="ditty-renderer">
          <Select
            id="ditty-renderer"
            value={profile.renderer}
            options={RENDERERS}
            onChange={(v) => onChange({ renderer: v as Profile["renderer"] })}
          />
        </Field>

        <RowField label="Copy on select">
          <Switch checked={profile.copyOnSelect} onChange={(v) => onChange({ copyOnSelect: v })} label="Copy on select" />
        </RowField>

        <RowField label="Bell notification">
          <Switch
            checked={profile.bellStyle === "visual"}
            onChange={(v) => onChange({ bellStyle: v ? "visual" : "none" })}
            label="Bell notification"
          />
        </RowField>

        <button
          type="button"
          onClick={onReset}
          className="rounded border border-line px-3 py-1.5 text-left text-sm text-muted-bright hover:text-[#E4E7EC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
        >
          Reset to server defaults
        </button>
      </div>
    </aside>
  );
}

function Field({ label, htmlFor, children }: { label: string; htmlFor?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={htmlFor} className="text-xs text-muted-bright">
        {label}
      </label>
      {children}
    </div>
  );
}

function RowField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-xs text-muted-bright">{label}</span>
      {children}
    </div>
  );
}

function Switch({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className={`flex h-5 w-9 shrink-0 items-center rounded-full p-0.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine focus-visible:ring-offset-1 focus-visible:ring-offset-ink-soft ${
        checked ? "justify-end bg-pine" : "justify-start bg-line"
      }`}
    >
      <span aria-hidden="true" className="h-4 w-4 shrink-0 rounded-full bg-white shadow" />
    </button>
  );
}
