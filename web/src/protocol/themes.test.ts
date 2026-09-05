import { describe, expect, it } from "vitest";
import { THEME_NAMES, THEMES, resolveChromeTint, resolveTheme } from "./themes";
import { defaultProfile } from "./defaultProfile";

describe("themes", () => {
  it("has exactly the six SPEC.md §7 themes", () => {
    expect(THEME_NAMES.sort()).toEqual(
      ["ditty-dark", "ditty-light", "nord", "dracula", "solarized-dark", "monokai"].sort(),
    );
  });

  it("resolveTheme falls back to ditty-dark for an unknown theme name", () => {
    const resolved = resolveTheme({ ...defaultProfile, theme: "not-a-theme" });
    expect(resolved).toEqual(THEMES["ditty-dark"].xterm);
  });

  it("resolveTheme applies profile.colors as an override on top of the named theme", () => {
    const resolved = resolveTheme({ ...defaultProfile, theme: "nord", colors: { background: "#000000" } });
    expect(resolved.background).toBe("#000000");
    expect(resolved.foreground).toBe(THEMES.nord.xterm.foreground);
  });

  it("resolveChromeTint matches the profile's theme", () => {
    expect(resolveChromeTint({ ...defaultProfile, theme: "dracula" })).toEqual(THEMES.dracula.chrome);
  });

  // Regression test: ChromeBar/StatusBar text used to be a hardcoded light
  // color that only worked against a dark chrome.bg — illegible against
  // ditty-light's near-white one. Every theme's fg/fgMuted must now have
  // real contrast against its own chrome.bg, not just against the dark
  // ones this bug happened to ship with.
  describe("chrome text contrast", () => {
    for (const name of THEME_NAMES) {
      const { bg, fg, fgMuted } = THEMES[name].chrome;
      it(`${name}: fg is readable against chrome.bg`, () => {
        expect(contrastRatio(fg, bg)).toBeGreaterThanOrEqual(4.5);
      });
      it(`${name}: fgMuted is readable against chrome.bg`, () => {
        expect(contrastRatio(fgMuted, bg)).toBeGreaterThanOrEqual(3);
      });
    }
  });
});

function hexToLinear(hex: string): [number, number, number] {
  const n = parseInt(hex.replace("#", ""), 16);
  const srgb = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((c) => c / 255);
  return srgb.map((c) => (c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4))) as [number, number, number];
}

function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToLinear(hex);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG 2.x contrast ratio between two hex colors, 1 (none) to 21 (max). */
function contrastRatio(a: string, b: string): number {
  const l1 = relativeLuminance(a) + 0.05;
  const l2 = relativeLuminance(b) + 0.05;
  return l1 > l2 ? l1 / l2 : l2 / l1;
}
