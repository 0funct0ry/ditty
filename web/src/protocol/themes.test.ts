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
});
