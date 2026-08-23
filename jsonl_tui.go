package main

// TUI decoration for JSONL transcript blocks -- extracted from the engine in
// Phase 1. The engine's JSONLParser produces plain-text Content unless a
// Decorate hook is wired; this file IS that hook for the terminal surface,
// reproducing the pre-extraction tview styling byte for byte. The HTML
// surface never reads transcript Content (it consumes *TranscriptData), so
// only TUI construction sites wire this (see decorateForTUI callers).

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// tuiDecoratePart styles one transcript part with tview markup.
func tuiDecoratePart(partType, content, meta string) string {
	switch partType {
	case "user":
		// User message: white text on gray background (chat bubble style)
		return fmt.Sprintf("[white:#303030]%s[-:-:-]", content)
	case "diff":
		filename := meta
		if idx := strings.LastIndex(meta, "/"); idx >= 0 {
			filename = meta[idx+1:]
		}
		return fmt.Sprintf("[#808080]--- %s ---[-]", filename) + "\n" + colorizeDiffLines(content)
	case "assistant":
		return formatAssistantContent(content, 0)
	case "question":
		return fmt.Sprintf("[yellow][?][-] %s", content)
	default:
		return content
	}
}

// decorateForTUI wires the tview decorator onto a JSONL parser headed for a
// terminal surface. Safe to call on any Parser; non-JSONL pass through.
func decorateForTUI(p Parser) Parser {
	if jp, ok := p.(*JSONLParser); ok {
		jp.Decorate = tuiDecoratePart
	}
	return p
}

// Additional inline pattern for function references
var codePatternBracket = regexp.MustCompile(`\[([^\]]+\(\))\]`) // [funcName()]

// formatAssistantContent applies markdown formatting plus function highlighting
func formatAssistantContent(text string, termWidth int) string {
	// First apply standard markdown formatting from formatter.go
	text = annotatedLinesToString(formatMarkdown(text, termWidth))
	// Then highlight [funcName()] patterns - yellow for function references
	text = codePatternBracket.ReplaceAllString(text, "[yellow]$1[-]")
	return text
}

// colorizeDiffLines applies tview color tags to diff lines for inline rendering
func colorizeDiffLines(diff string) string {
	var sb strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Added line - green background
			sb.WriteString(fmt.Sprintf("[white:#2d5a2d]%s[-:-]\n", line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// Removed line - magenta background
			sb.WriteString(fmt.Sprintf("[white:#5a2d5a]%s[-:-]\n", line))
		} else if strings.HasPrefix(line, "@@") {
			// Hunk header - dim
			sb.WriteString(fmt.Sprintf("[#808080]%s[-]\n", line))
		} else if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			// Skip file headers (we have our own header)
			continue
		} else {
			// Context line
			sb.WriteString(line + "\n")
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// surfaceEngineWarnings prints non-fatal parser warnings (e.g. video too
// large to inline) that the engine records on Block.Data instead of writing
// to stderr itself.
func surfaceEngineWarnings(blocks []Block) {
	for i := range blocks {
		if vd, ok := blocks[i].Data.(*VideoData); ok && vd.Warning != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s.\n", vd.Warning)
		}
	}
}
