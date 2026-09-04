package profile

// The six themes from SPEC.md §7. Full 16-color ANSI palettes live only in
// the frontend theme registry (web/src/protocol/themes.ts) — the browser
// needs no fetch, so there is nothing to seed from the server beyond the
// name itself. This file exists so --profile-theme can be validated without
// duplicating the palettes here.
const (
	ThemeDittyDark     = "ditty-dark"
	ThemeDittyLight    = "ditty-light"
	ThemeNord          = "nord"
	ThemeDracula       = "dracula"
	ThemeSolarizedDark = "solarized-dark"
	ThemeMonokai       = "monokai"
)

// Themes lists every valid Profile.Theme value, in the order they appear in
// the settings drawer's swatch grid.
func Themes() []string {
	return []string{
		ThemeDittyDark,
		ThemeDittyLight,
		ThemeNord,
		ThemeDracula,
		ThemeSolarizedDark,
		ThemeMonokai,
	}
}

// ValidTheme reports whether name is one of the six canonical themes.
func ValidTheme(name string) bool {
	for _, t := range Themes() {
		if t == name {
			return true
		}
	}
	return false
}
