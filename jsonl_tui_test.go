package main

// TUI byte-parity for the Phase 1 Decorate extraction. The golden harness
// covers the HTML surface (which never reads transcript Content); this pins
// the TERMINAL surface: CreateTurnBlock with the tview decorator wired must
// produce the exact bytes the pre-extraction inline switch produced. The
// expected strings below are copied verbatim from the pre-refactor
// parser_jsonl.go switch arms -- if the decorator drifts, this reds against
// history, not against itself.

import (
	"strings"
	"testing"
)

func turnOf(parts ...TurnPart) *ConversationTurn {
	return &ConversationTurn{Parts: parts}
}

func TestTUIDecoration_MatchesPreExtractionBytes(t *testing.T) {
	p := &JSONLParser{Decorate: tuiDecoratePart}

	block := p.CreateTurnBlock(turnOf(
		TurnPart{Type: "user", Content: "hello there"},
		TurnPart{Type: "question", Content: "proceed?"},
		TurnPart{Type: "tool_result", Content: "raw tool output"},
	), 1)

	// Pre-extraction bytes, verbatim from the old switch:
	if !strings.Contains(block.Content, "[white:#303030]hello there[-:-:-]") {
		t.Errorf("user part lost its chat-bubble styling: %q", block.Content)
	}
	if !strings.Contains(block.Content, "[yellow][?][-] proceed?") {
		t.Errorf("question part lost its styling: %q", block.Content)
	}
	if !strings.Contains(block.Content, "raw tool output") {
		t.Errorf("tool_result must pass through untouched: %q", block.Content)
	}
}

func TestTUIDecoration_DiffPartHeaderAndColor(t *testing.T) {
	p := &JSONLParser{Decorate: tuiDecoratePart}
	block := p.CreateTurnBlock(turnOf(
		TurnPart{Type: "diff", Content: "-old line\n+new line", Meta: "src/deep/file.go"},
	), 1)

	// Header uses the BASENAME with the gray tview tag, exactly as before.
	if !strings.Contains(block.Content, "[#808080]--- file.go ---[-]") {
		t.Errorf("diff header drifted: %q", block.Content)
	}
	// Body goes through colorizeDiffLines (colorized, not raw).
	if strings.Contains(block.Content, "\n-old line\n") {
		t.Errorf("diff lines were not colorized: %q", block.Content)
	}
}

func TestPlainDecoration_NoTviewMarkup(t *testing.T) {
	// The nil-Decorate default (what a host sees) must carry NO tview tags.
	p := &JSONLParser{}
	block := p.CreateTurnBlock(turnOf(
		TurnPart{Type: "user", Content: "hello"},
		TurnPart{Type: "diff", Content: "+x", Meta: "a/b.go"},
		TurnPart{Type: "question", Content: "ok?"},
	), 1)

	for _, tag := range []string{"[white:", "[yellow]", "[#808080]", "[-:-:-]"} {
		if strings.Contains(block.Content, tag) {
			t.Errorf("plain (host-facing) content carries tview markup %q: %q", tag, block.Content)
		}
	}
	if !strings.Contains(block.Content, "--- b.go ---") {
		t.Errorf("plain diff header missing: %q", block.Content)
	}
	if !strings.Contains(block.Content, "[?] ok?") {
		t.Errorf("plain question marker missing: %q", block.Content)
	}
}
