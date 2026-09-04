import { useEffect, useRef, useState } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import "@xterm/xterm/css/xterm.css";
import type { Profile } from "../protocol/types";

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

function applyProfile(term: XTerm, profile: Profile) {
  term.options.fontFamily = profile.fontFamily;
  term.options.fontSize = profile.fontSize;
  term.options.lineHeight = profile.lineHeight;
  term.options.cursorStyle = profile.cursorStyle;
  term.options.cursorBlink = profile.cursorBlink;
  term.options.scrollback = profile.scrollback;
  term.options.rightClickSelectsWord = profile.rightClickPaste;
  term.options.macOptionIsMeta = profile.macOptionIsMeta;
  if (Object.keys(profile.colors).length > 0) {
    term.options.theme = profile.colors;
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
  const [rendererFallback, setRendererFallback] = useState(false);

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
      term.loadAddon(new WebglAddon());
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

    termRef.current = term;
    fitRef.current = fit;
    writeRef.current = (data) => term.write(data);

    const resizeObserver = new ResizeObserver(() => {
      fit.fit();
      if (sizingRef.current) {
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
    };
    // Mount once; the container ref itself never changes for this component's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (termRef.current && profile) {
      applyProfile(termRef.current, profile);
    }
  }, [profile]);

  return (
    <div className="relative h-full w-full bg-ink-deep">
      <div ref={containerRef} className="h-full w-full" />
      {rendererFallback && (
        <div className="pointer-events-none absolute bottom-2 right-2 rounded bg-ink-soft px-2 py-1 font-mono text-xs text-muted-bright">
          renderer: canvas (WebGL unavailable)
        </div>
      )}
    </div>
  );
}
