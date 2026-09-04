package profile

import (
	"encoding/json"
	"testing"
)

func TestDefault(t *testing.T) {
	got := Default()
	want := Profile{
		Theme:           "ditty-dark",
		FontFamily:      "JetBrains Mono, SF Mono, Menlo, monospace",
		FontSize:        14,
		LineHeight:      1.2,
		CursorStyle:     "block",
		CursorBlink:     true,
		Scrollback:      5000,
		Renderer:        "webgl",
		BellStyle:       "none",
		MacOptionIsMeta: true,
		AltSendsEscape:  true,
		CopyOnSelect:    true,
		RightClickPaste: false,
		UnicodeVersion:  "11",
		EastAsianWidth:  false,
		Colors:          map[string]string{},
	}

	if got.Theme != want.Theme || got.FontFamily != want.FontFamily || got.FontSize != want.FontSize ||
		got.LineHeight != want.LineHeight || got.CursorStyle != want.CursorStyle || got.CursorBlink != want.CursorBlink ||
		got.Scrollback != want.Scrollback || got.Renderer != want.Renderer || got.BellStyle != want.BellStyle ||
		got.MacOptionIsMeta != want.MacOptionIsMeta || got.AltSendsEscape != want.AltSendsEscape ||
		got.CopyOnSelect != want.CopyOnSelect || got.RightClickPaste != want.RightClickPaste ||
		got.UnicodeVersion != want.UnicodeVersion || got.EastAsianWidth != want.EastAsianWidth ||
		len(got.Colors) != len(want.Colors) {
		t.Fatalf("Default() = %+v, want %+v", got, want)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	p := Default()
	raw, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Profile
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Theme != p.Theme || got.FontSize != p.FontSize {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, p)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}
	for _, field := range []string{
		"theme", "fontFamily", "fontSize", "lineHeight", "cursorStyle", "cursorBlink",
		"scrollback", "renderer", "bellStyle", "macOptionIsMeta", "altSendsEscape",
		"copyOnSelect", "rightClickPaste", "unicodeVersion", "eastAsianWidth", "colors",
	} {
		if _, ok := m[field]; !ok {
			t.Errorf("marshalled Profile missing JSON field %q", field)
		}
	}
}

func TestValidTheme(t *testing.T) {
	for _, name := range Themes() {
		if !ValidTheme(name) {
			t.Errorf("ValidTheme(%q) = false, want true", name)
		}
	}
	if ValidTheme("not-a-theme") {
		t.Error("ValidTheme(\"not-a-theme\") = true, want false")
	}
	if len(Themes()) != 6 {
		t.Errorf("Themes() has %d entries, want 6", len(Themes()))
	}
}
