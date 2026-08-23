package engine

// Package engine is aster's rendering pipeline: Parse -> []Block -> Render.
//
// It is consumed two ways:
//   - by the aster CLI (the original and reference consumer), via the aliases
//     bridge in the root package;
//   - by hosts that embed rendered content in their OWN page shell --
//     tunnel-artifacts' renderByType is the intended first host (Phase 2 of
//     the merge). Hosts call Parse/ParseFile + RenderFragment and place
//     Assets() in their <head>; they must NOT use RenderPage, which emits a
//     complete standalone document.
//
// HOST INVARIANT -- static content only: always parse with static=true
// (ParseFile's second argument). Static mode inlines images/video as data
// URIs. Non-static mode emits /asset/... URLs that exist only on the CLI's
// local server -- inside a sandboxed iframe on a doc host they 404. The CLI
// exercises both modes; a host must never see the non-static one.
//
// This file is the API surface. Everything else in the package is
// implementation; unexported symbols are not a contract.

import (
	"os"
	"path/filepath"
)

// Options controls rendering.
type Options struct {
	// ShowLineNumbers renders a source line-number gutter where supported.
	ShowLineNumbers bool
}

// Parse parses content already typed by the caller. typeName is one of the
// FileTypes keys ("md", "html", "csv", "diff", "jsonl", "json", "yaml",
// "txt") or "" to detect from the content itself. HTML converts to markdown
// first, exactly as every aster surface does.
func Parse(content string, typeName string) []Block {
	var p Parser
	switch typeName {
	case "md":
		p = &MarkdownParser{}
	case "html":
		p = &HTMLParser{}
	case "jsonl":
		p = &JSONLParser{}
	case "diff":
		p = &DiffParser{}
	case "csv":
		p = &CsvParser{}
	case "json":
		p = &TodoParser{}
	case "txt", "yaml":
		p = &TxtParser{}
	default:
		p = DetectParserFromContent(content)
	}
	if _, isHTML := p.(*HTMLParser); isHTML {
		md, _ := HTMLToMarkdown(content)
		content = md
		p = &MarkdownParser{}
	}
	return p.Parse(content)
}

// ParseFile detects the file's type and parses it. static=true inlines
// binary assets as data URIs (REQUIRED for hosts; see the invariant above).
func ParseFile(path string, static bool) ([]Block, error) {
	ft := DetectFileType(path)
	if ft == "img" {
		return (&ImageParser{}).ParseFile(path, static)
	}
	if ft == "vid" {
		return (&VideoParser{}).ParseFile(path, static)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	p := DetectParser(path)
	if _, isHTML := p.(*HTMLParser); isHTML {
		md, _ := HTMLToMarkdown(content)
		content = md
		p = &MarkdownParser{}
	}
	return p.Parse(content), nil
}

// RenderPage renders blocks as a complete, self-contained HTML document
// (inline CSS + highlight.js, no external fetches). This is what the CLI's
// --html/--share/push surfaces emit. Hosts with their own page shell use
// RenderFragment instead.
func RenderPage(title string, blocks []Block, opts Options) string {
	return RenderStaticHTMLPage(title, blocks, opts.ShowLineNumbers)
}

// RenderFragment renders blocks as body-level HTML with no <html>, <head>,
// <body>, or <style> wrapper -- the form a host embeds inside its own page
// shell. Pair with Assets() in the host's <head>.
func RenderFragment(blocks []Block, opts Options) string {
	var sb []byte
	single := len(blocks) == 1
	for i := range blocks {
		sb = append(sb, formatBlockHTML(&blocks[i], opts.ShowLineNumbers, single)...)
	}
	return string(sb)
}

// Assets returns the CSS and JS a host must place in its <head> for
// RenderFragment output to style and highlight correctly. Both are inline
// text (no URLs): css is the engine stylesheet plus highlight.js theme, js
// is highlight.js itself.
func Assets() (css string, js string) {
	return highlightCSS + "\n" + cssStyles(), highlightJS
}

// PageTitle derives a title for a file the way the CLI does: frontmatter
// title wins, else the base filename. (HTML <title> extraction happens in
// HTMLToMarkdown's second return; callers using Parse on HTML handle that.)
func PageTitle(path string, content string) string {
	fm, _ := ParseFrontmatter(content)
	if fm.Title != "" {
		return fm.Title
	}
	return filepath.Base(path)
}

// Exported bridges for symbols the CLI shares with the engine.

// ImageExtensions and VideoExtensions are the file extensions the registry
// treats as image/video content.
var ImageExtensions = imageExtensions
var VideoExtensions = videoExtensions

// IsTableLine, IsTableSeparator, and ParseTableCells are the markdown-table
// primitives shared by the HTML renderer and the CLI's TUI formatter.
func IsTableLine(line string) bool        { return isTableLine(line) }
func IsTableSeparator(line string) bool   { return isTableSeparator(line) }
func ParseTableCells(line string) []string { return parseTableCells(line) }

// HasStructuredPatch and ExtractStructuredPatch expose the transcript
// patch-detection primitives the CLI's follow mode shares with the parser.
func HasStructuredPatch(msg map[string]interface{}) bool { return hasStructuredPatch(msg) }
func ExtractStructuredPatch(msg map[string]interface{}) string { return extractStructuredPatch(msg) }
