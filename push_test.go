package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The push contract is "same file -> same key -> same URL". Key derivation is
// where that promise lives client-side, so it gets pinned hardest.

func TestPushKeyIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "out")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "report.html")
	if err := os.WriteFile(file, []byte("<p>hi</p>"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Relative and absolute spellings of the same file map to the same key.
	rel := pushKey(filepath.Join("out", "report.html"))
	abs := pushKey(file)
	if rel != abs {
		t.Errorf("relative vs absolute spelling diverged: %q vs %q", rel, abs)
	}
	// Replay is identical.
	if again := pushKey(file); again != abs {
		t.Errorf("key flapped across calls: %q vs %q", abs, again)
	}
	// Shape: aster/<project>/<relpath>, slash-normalized.
	want := "aster/" + filepath.Base(dir) + "/out/report.html"
	if abs != want {
		t.Errorf("key = %q, want %q", abs, want)
	}
}

func TestPushKeyOutsideCwdFallsBack(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	file := filepath.Join(other, "chart.svg")
	if err := os.WriteFile(file, []byte("<svg/>"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	key := pushKey(file)
	want := "aster/" + filepath.Base(other) + "/chart.svg"
	if key != want {
		t.Errorf("outside-cwd key = %q, want %q", key, want)
	}
}

// Verbatim vs rendered: the three native tunl types travel untouched; every
// rendered type arrives as a self-contained HTML page.
func TestRenderForPushTypeRouting(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	cases := []struct {
		file     string
		content  string
		wantType string
		verbatim bool
	}{
		{"page.html", "<!doctype html><title>t</title><p>generated</p>", "html", true},
		{"chart.svg", `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`, "svg", true},
		{"notes.md", "# Heading\n\nbody", "markdown", true},
		{"data.csv", "a,b\n1,2\n", "html", false},
		{"changes.diff", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n", "html", false},
	}
	for _, tc := range cases {
		p := write(tc.file, tc.content)
		got, gotType, err := renderForPush(p)
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		if gotType != tc.wantType {
			t.Errorf("%s: type = %q, want %q", tc.file, gotType, tc.wantType)
		}
		if tc.verbatim && got != tc.content {
			t.Errorf("%s: verbatim rail modified content", tc.file)
		}
		if !tc.verbatim && !strings.Contains(got, "<!DOCTYPE html>") {
			t.Errorf("%s: rendered rail did not produce a full HTML page", tc.file)
		}
	}
}

// A raster image arrives as rendered HTML with its bytes inlined as a data
// URI -- self-contained, so it works inside the doc host's opaque-origin
// sandbox with no /asset/ routes to 404.
func TestRenderForPushInlinesImages(t *testing.T) {
	// 1x1 transparent PNG.
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	p := filepath.Join(t.TempDir(), "dot.png")
	if err := os.WriteFile(p, png, 0644); err != nil {
		t.Fatal(err)
	}
	got, gotType, err := renderForPush(p)
	if err != nil {
		t.Fatal(err)
	}
	if gotType != "html" {
		t.Errorf("type = %q, want html", gotType)
	}
	if !strings.Contains(got, "data:image/png;base64,") {
		t.Error("image bytes not inlined as a data URI")
	}
}

func TestLoadPushConfigEnvPrecedence(t *testing.T) {
	t.Setenv("ASTER_PUSH_URL", "https://staging.example.com/")
	t.Setenv("ASTER_PUSH_TOKEN", "tunl_at_explicit")
	t.Setenv("TUNL_TOKEN", "tunl_at_fallback")

	cfg := loadPushConfig()
	if cfg.URL != "https://staging.example.com" {
		t.Errorf("URL = %q (trailing slash should be trimmed)", cfg.URL)
	}
	if cfg.Token != "tunl_at_explicit" {
		t.Errorf("ASTER_PUSH_TOKEN should win over TUNL_TOKEN, got %q", cfg.Token)
	}

	t.Setenv("ASTER_PUSH_TOKEN", "")
	cfg = loadPushConfig()
	if cfg.Token != "tunl_at_fallback" {
		t.Errorf("TUNL_TOKEN fallback broken, got %q", cfg.Token)
	}
}
