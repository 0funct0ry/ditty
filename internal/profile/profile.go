// Package profile implements the Profile from SPEC.md §7: per-Client
// terminal appearance and behaviour, seeded server-side and sent as part of
// Hello, then merged with any browser-persisted copy. Field names and JSON
// tags must match web/src/protocol/types.ts's Profile interface exactly —
// there is no shared codegen, so the two are kept in sync by hand (the same
// convention that file's own doc comment states).
package profile

import "encoding/json"

// Profile is the SPEC.md §7 Profile shape.
type Profile struct {
	Theme           string            `json:"theme"`
	FontFamily      string            `json:"fontFamily"`
	FontSize        int               `json:"fontSize"`
	LineHeight      float64           `json:"lineHeight"`
	CursorStyle     string            `json:"cursorStyle"`
	CursorBlink     bool              `json:"cursorBlink"`
	Scrollback      int               `json:"scrollback"`
	Renderer        string            `json:"renderer"`
	BellStyle       string            `json:"bellStyle"`
	MacOptionIsMeta bool              `json:"macOptionIsMeta"`
	AltSendsEscape  bool              `json:"altSendsEscape"`
	CopyOnSelect    bool              `json:"copyOnSelect"`
	RightClickPaste bool              `json:"rightClickPaste"`
	UnicodeVersion  string            `json:"unicodeVersion"`
	EastAsianWidth  bool              `json:"eastAsianWidth"`
	Colors          map[string]string `json:"colors"`
}

// Cursor style values valid in Profile.CursorStyle.
const (
	CursorStyleBlock     = "block"
	CursorStyleUnderline = "underline"
	CursorStyleBar       = "bar"
)

// Renderer kinds valid in Profile.Renderer.
const (
	RendererWebGL  = "webgl"
	RendererCanvas = "canvas"
)

// Bell styles valid in Profile.BellStyle.
const (
	BellStyleNone         = "none"
	BellStyleSound        = "sound"
	BellStyleNotification = "visual"
)

// Default returns the SPEC.md §7 default Profile, matching
// web/src/protocol/defaultProfile.ts field for field.
func Default() Profile {
	return Profile{
		Theme:           ThemeDittyDark,
		FontFamily:      "JetBrains Mono, SF Mono, Menlo, monospace",
		FontSize:        14,
		LineHeight:      1.2,
		CursorStyle:     CursorStyleBlock,
		CursorBlink:     true,
		Scrollback:      5000,
		Renderer:        RendererWebGL,
		BellStyle:       BellStyleNone,
		MacOptionIsMeta: true,
		AltSendsEscape:  true,
		CopyOnSelect:    true,
		RightClickPaste: false,
		UnicodeVersion:  "11",
		EastAsianWidth:  false,
		Colors:          map[string]string{},
	}
}

// Marshal encodes p as the json.RawMessage carried in wire.Hello.Profile.
func (p Profile) Marshal() (json.RawMessage, error) {
	return json.Marshal(p)
}
