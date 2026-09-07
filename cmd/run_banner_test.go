package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestBannerColorEnabled(t *testing.T) {
	var buf bytes.Buffer
	if bannerColorEnabled(&buf) {
		t.Fatal("bannerColorEnabled(bytes.Buffer) = true, want false (not a terminal)")
	}

	t.Setenv("NO_COLOR", "1")
	if bannerColorEnabled(&buf) {
		t.Fatal("bannerColorEnabled with NO_COLOR set = true, want false")
	}
}

func TestNewBannerColors(t *testing.T) {
	off := newBannerColors(false)
	if off != (bannerColors{}) {
		t.Fatalf("newBannerColors(false) = %+v, want every field empty", off)
	}

	on := newBannerColors(true)
	if on.reset == "" || on.wordmark == "" || on.dim == "" || on.url == "" {
		t.Fatalf("newBannerColors(true) = %+v, want every field non-empty", on)
	}
}

func TestPrintBanner_NoColor(t *testing.T) {
	var buf bytes.Buffer
	printBanner(&buf, bannerInfo{
		version:   "v1.0.0",
		sessionID: "5a72427b8c6ad5c1",
		command:   "htop",
		url:       "http://127.0.0.1:7654/t/abc123",
		access:    "read-only · token · 0 clients",
	})
	out := buf.String()

	for _, want := range []string{
		"ditty v1.0.0",
		"session   5a72427b8c6ad5c1 (htop)",
		"url       http://127.0.0.1:7654/t/abc123",
		"access    read-only · token · 0 clients",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("banner output missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("banner output contains an ANSI escape sequence when color is disabled; got:\n%s", out)
	}
}
