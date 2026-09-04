import { useEffect, useRef, useState } from "react";

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps {
  id?: string;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  "aria-label"?: string;
}

/** A fully Tailwind-styled listbox, replacing the native `<select>` (which
 * renders its dropdown in browser chrome the settings drawer can't style).
 * No external dependency — a small controlled popover is enough for the
 * three fixed option sets this drawer needs. */
export function Select({ id, value, options, onChange, "aria-label": ariaLabel }: SelectProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const selected = options.find((o) => o.value === value) ?? options[0];

  useEffect(() => {
    if (!open) return;
    const handleClick = (ev: MouseEvent) => {
      if (!containerRef.current?.contains(ev.target as Node)) setOpen(false);
    };
    const handleKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") setOpen(false);
    };
    window.addEventListener("mousedown", handleClick);
    window.addEventListener("keydown", handleKey);
    return () => {
      window.removeEventListener("mousedown", handleClick);
      window.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  return (
    <div ref={containerRef} className="relative">
      <button
        id={id}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between rounded border border-line bg-ink px-2 py-1.5 text-left text-sm text-[#E4E7EC] hover:border-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pine"
      >
        <span className="truncate">{selected?.label}</span>
        <ChevronIcon className={`shrink-0 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <ul
          role="listbox"
          aria-label={ariaLabel}
          className="absolute left-0 right-0 top-full z-20 mt-1 max-h-48 overflow-auto rounded border border-line bg-ink-soft py-1 shadow-lg"
        >
          {options.map((opt) => {
            const isSelected = opt.value === value;
            return (
              <li key={opt.value} role="option" aria-selected={isSelected}>
                <button
                  type="button"
                  onClick={() => {
                    onChange(opt.value);
                    setOpen(false);
                  }}
                  className={`flex w-full items-center justify-between px-3 py-1.5 text-left text-sm hover:bg-ink focus-visible:outline-none focus-visible:bg-ink ${
                    isSelected ? "text-[#E4E7EC]" : "text-muted-bright"
                  }`}
                >
                  <span className="truncate">{opt.label}</span>
                  {isSelected && <CheckIcon />}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

function ChevronIcon({ className = "" }: { className?: string }) {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" className={className}>
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" className="shrink-0 text-pine">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}
