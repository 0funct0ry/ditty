import { useEffect, useRef, useState } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import "@xterm/xterm/css/xterm.css";
import type { Profile } from "../protocol/types";
import { resolveTheme } from "../protocol/themes";

// Keys xterm.js must never capture, so the browser/OS keeps its own
// behaviour for them (SPEC.md §7.1).
const NEVER_CAPTURE = new Set(["F11", "F5"]);

function isNeverCaptured(ev: KeyboardEvent): boolean {
  if (NEVER_CAPTURE.has(ev.key)) return true;
  const mod = ev.metaKey || ev.ctrlKey;
  if (mod && (ev.key === "+" || ev.key === "-" || ev.key === "0")) return true; // browser zoom
  if (mod && ev.key.toLowerCase() === "r" && ev.shiftKey) return true; // hard reload
  return false;
}

// xterm.js has no first-class altSendsEscape option — Alt-prefixed input
// already reaches the PTY as ESC-prefixed bytes by default, which is what
// that Profile field describes, so there is nothing to toggle on the
// xterm.Terminal itself.
function applyProfile(term: XTerm, profile: Profile) {
  term.options.fontFamily = profile.fontFamily;
  term.options.fontSize = profile.fontSize;
  term.options.lineHeight = profile.lineHeight;
  term.options.cursorStyle = profile.cursorStyle;
  term.options.cursorBlink = profile.cursorBlink;
  term.options.scrollback = profile.scrollback;
  term.options.rightClickSelectsWord = profile.rightClickPaste;
  term.options.macOptionIsMeta = profile.macOptionIsMeta;
  term.options.theme = resolveTheme(profile);
  if (profile.unicodeVersion === "11") {
    term.unicode.activeVersion = "11";
  }
}

export interface TerminalProps {
  profile: Profile | null;
  writable: boolean;
  sizing: boolean;
  /** Imperative write target — App feeds Output frame bytes in here directly. */
  writeRef: React.MutableRefObject<((data: Uint8Array) => void) | null>;
  onInput: (data: Uint8Array) => void;
  onResize: (cols: number, rows: number) => void;
  onReadOnlyKeystroke: () => void;
}

export function Terminal({ profile, writable, sizing, writeRef, onInput, onResize, onReadOnlyKeystroke }: TerminalProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const webglAddonRef = useRef<WebglAddon | null>(null);
  const [rendererFallback, setRendererFallback] = useState(false);
  const [bellFlash, setBellFlash] = useState(false);

  // writable/sizing/onInput/onResize/onReadOnlyKeystroke are read through
  // refs inside the mount effect below so the terminal is created exactly
  // once and never torn down on every prop change.
  const writableRef = useRef(writable);
  writableRef.current = writable;
  const sizingRef = useRef(sizing);
  sizingRef.current = sizing;
  const onInputRef = useRef(onInput);
  onInputRef.current = onInput;
  const onResizeRef = useRef(onResize);
  onResizeRef.current = onResize;
  const onReadOnlyKeystrokeRef = useRef(onReadOnlyKeystroke);
  onReadOnlyKeystrokeRef.current = onReadOnlyKeystroke;
  const bellStyleRef = useRef<Profile["bellStyle"]>(profile?.bellStyle ?? "none");
  bellStyleRef.current = profile?.bellStyle ?? "none";
  const copyOnSelectRef = useRef(profile?.copyOnSelect ?? true);
  copyOnSelectRef.current = profile?.copyOnSelect ?? true;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const term = new XTerm({
      allowProposedApi: true,
      fontFamily: "JetBrains Mono, SF Mono, Menlo, monospace",
      fontSize: 14,
      cursorBlink: true,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.loadAddon(new Unicode11Addon());
    term.unicode.activeVersion = "11";

    try {
      const webgl = new WebglAddon();
      term.loadAddon(webgl);
      webglAddonRef.current = webgl;
    } catch {
      setRendererFallback(true);
    }

    term.open(container);
    fit.fit();

    term.attachCustomKeyEventHandler((ev) => {
      if (isNeverCaptured(ev)) return false; // let the browser handle it
      return true;
    });

    term.onData((data) => {
      if (!writableRef.current) {
        onReadOnlyKeystrokeRef.current();
        return;
      }
      onInputRef.current(new TextEncoder().encode(data));
    });

    term.onSelectionChange(() => {
      if (!copyOnSelectRef.current) return;
      const selection = term.getSelection();
      if (selection) void navigator.clipboard.writeText(selection).catch(() => {});
    });

    term.onBell(() => {
      if (bellStyleRef.current === "visual") {
        setBellFlash(true);
        setTimeout(() => setBellFlash(false), 150);
      }
      // "sound" relies on the BEL byte itself, which xterm.js does not play
      // audibly by default; a real bell sound is out of scope for M6's
      // fixture-only rehearsal and is left for a later milestone to wire an
      // <audio> element behind this same branch if it's still wanted.
    });

    termRef.current = term;
    fitRef.current = fit;
    writeRef.current = (data) => term.write(data);

    const resizeObserver = new ResizeObserver(() => {
      fit.fit();
      // A container mid-reflow (e.g. before fonts/layout have settled) can
      // make FitAddon compute a degenerate 0-or-negative size for a single
      // observation; never forward that to the PTY — SPEC.md §5.1's own
      // clamp floor (10x5) would mask it as a small-but-valid Resize rather
      // than surfacing the real, still-correct container size once layout
      // finishes settling.
      if (sizingRef.current && term.cols > 0 && term.rows > 0) {
        onResizeRef.current(term.cols, term.rows);
      }
    });
    resizeObserver.observe(container);

    return () => {
      resizeObserver.disconnect();
      writeRef.current = null;
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      webglAddonRef.current = null;
    };
    // Mount once; the container ref itself never changes for this component's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (termRef.current && profile) {
      applyProfile(termRef.current, profile);
    }
  }, [profile]);

  // Becoming the sizing Client (on Hello, or on a sizing handover) is not a
  // physical container resize, so ResizeObserver never fires for it on its
  // own — without this, the PTY stays at whatever size Spawn defaulted to
  // (SPEC.md §5.4) until the browser window happens to resize later. Fit and
  // send the terminal's already-correct computed size the moment sizing is
  // granted. This can land in the same tick as first mount/Hello, before the
  // browser has finished a layout+paint pass for a freshly-opened tab (fonts
  // still loading, initial reflow not settled) — FitAddon.fit() can compute
  // a degenerate 0-or-tiny size in that instant, which would otherwise get
  // sent, clamped server-side to SPEC.md §5.1's floor, and silently stick
  // for the rest of the session with nothing to correct it (a container
  // whose real pixel size never changes again never re-fires
  // ResizeObserver). Defer past two animation frames — one for layout, one
  // for paint — before trusting the computed size, and never send a
  // non-positive one regardless.
  useEffect(() => {
    if (!sizing) return;
    let cancelled = false;
    const rafHandles: number[] = [];
    rafHandles.push(
      requestAnimationFrame(() => {
        rafHandles.push(
          requestAnimationFrame(() => {
            if (cancelled) return;
            const term = termRef.current;
            const fit = fitRef.current;
            if (!term || !fit) return;
            fit.fit();
            if (term.cols > 0 && term.rows > 0) {
              onResizeRef.current(term.cols, term.rows);
            }
          }),
        );
      }),
    );
    return () => {
      cancelled = true;
      rafHandles.forEach((h) => cancelAnimationFrame(h));
    };
  }, [sizing]);

  // Renderer switch: swap the WebGL addon in/out without touching layout —
  // this must never trigger a fit()/Resize, since a settings change should
  // not resize the PTY (SPEC.md §10.2 acceptance).
  useEffect(() => {
    const term = termRef.current;
    if (!term || !profile) return;
    const wantWebgl = profile.renderer === "webgl";
    const hasWebgl = webglAddonRef.current !== null;
    if (wantWebgl === hasWebgl) return;

    if (wantWebgl) {
      try {
        const webgl = new WebglAddon();
        term.loadAddon(webgl);
        webglAddonRef.current = webgl;
        setRendererFallback(false);
      } catch {
        setRendererFallback(true);
      }
    } else {
      webglAddonRef.current?.dispose();
      webglAddonRef.current = null;
    }
  }, [profile?.renderer]);

  return (
    <div className="relative h-full w-full bg-[var(--ditty-terminal-bg)]">
      <div ref={containerRef} className="h-full w-full" />
      {bellFlash && <div className="pointer-events-none absolute inset-0 bg-white/10" />}
      {rendererFallback && (
        <div className="pointer-events-none absolute bottom-2 right-2 rounded bg-ink-soft px-2 py-1 font-mono text-xs text-muted-bright">
          renderer: canvas (WebGL unavailable)
        </div>
      )}
    </div>
  );
}
