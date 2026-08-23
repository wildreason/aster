package engine

// Registry: the content-type map and parser detection. Moved from the CLI in
// Phase 1 -- which formats exist and how they are recognized is engine
// knowledge; the CLI (and any host) consumes it through the exported API in
// engine.go / the CLI's aliases bridge.

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// FileType defines a supported file type with its extensions
type FileType struct {
	Name       string
	Extensions []string
}

var FileTypes = map[string]FileType{
	"md":    {Name: "markdown", Extensions: []string{".md", ".markdown"}},
	"img":   {Name: "image", Extensions: imageExtensions},
	"vid":   {Name: "video", Extensions: videoExtensions},
	"txt":   {Name: "text", Extensions: []string{".txt", ".log"}},
	"json":  {Name: "json", Extensions: []string{".json"}},
	"yaml":  {Name: "yaml", Extensions: []string{".yaml", ".yml"}},
	"diff":  {Name: "diff", Extensions: []string{".diff", ".patch"}},
	"jsonl": {Name: "jsonl", Extensions: []string{".jsonl"}},
	"csv":   {Name: "csv", Extensions: []string{".csv", ".tsv"}},
	"html":  {Name: "html", Extensions: []string{".html", ".htm", ".xhtml"}},
}

// DetectFileType returns the type key for a file path based on extension
func DetectFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	for key, ft := range FileTypes {
		for _, e := range ft.Extensions {
			if ext == e {
				return key
			}
		}
	}
	return ""
}

// DetectParser selects the appropriate parser based on file extension
func DetectParser(filePath string) Parser {
	parsers := []Parser{
		&TodoParser{},
		&DiffParser{},
		&CsvParser{},
		&ContractParser{},
		&HTMLParser{},
		&MarkdownParser{},
		&JSONLParser{},
		&TxtParser{},
	}

	for _, parser := range parsers {
		if parser.Detect(filePath) {
			return parser
		}
	}

	return &MarkdownParser{}
}

// DetectParserFromContent tries to detect parser type from content (for stdin)
func DetectParserFromContent(content string) Parser {
	if DetectBlockContentType(content) == BlockContentDiff {
		return &DiffParser{}
	}

	// HTML -> converted to markdown downstream
	if isHTMLContent(content) {
		return &HTMLParser{}
	}

	// Count JSON lines to distinguish JSONL from single JSON
	lines := strings.Split(content, "\n")
	jsonLineCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var testJSON map[string]interface{}
		if err := json.Unmarshal([]byte(line), &testJSON); err == nil {
			jsonLineCount++
		} else {
			break
		}
	}
	if jsonLineCount >= 2 {
		return &JSONLParser{}
	}

	// JSON object/array -> plain text (preserves structure)
	if isJSON(content) {
		return &TxtParser{}
	}

	// YAML -> plain text (preserves structure)
	if isYAML(content) {
		return &TxtParser{}
	}

	// CSV -> table
	if isCSV(content) {
		return &CsvParser{}
	}

	return &MarkdownParser{}
}

var imageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".ico", ".svg"}

var videoExtensions = []string{".mp4", ".webm", ".mov", ".mkv"}

// ImageMIME returns the MIME type for an image extension
func ImageMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// VideoMIME returns the MIME type for a video extension
func VideoMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
