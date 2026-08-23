package engine

import (
	"strings"
	"testing"
)

// --- URL sanitization ---

func TestSanitizeURL_BlocksJavascript(t *testing.T) {
	dangerous := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"JAVASCRIPT:ALERT(1)",
		"javascript:void(0)",
		"  javascript:alert(1)",
	}
	for _, u := range dangerous {
		if result := sanitizeURL(u); result != "#" {
			t.Errorf("sanitizeURL(%q) = %q, want %q", u, result, "#")
		}
	}
}

func TestSanitizeURL_BlocksDataAndVbscript(t *testing.T) {
	dangerous := []string{
		"data:text/html,<script>alert(1)</script>",
		"DATA:text/html,payload",
		"vbscript:MsgBox(1)",
		"VBSCRIPT:payload",
	}
	for _, u := range dangerous {
		if result := sanitizeURL(u); result != "#" {
			t.Errorf("sanitizeURL(%q) = %q, want %q", u, result, "#")
		}
	}
}

func TestSanitizeURL_AllowsSafeSchemes(t *testing.T) {
	safe := []string{
		"https://example.com",
		"http://example.com",
		"mailto:user@example.com",
		"#section-1",
		"/relative/path",
		"../file.md",
	}
	for _, u := range safe {
		if result := sanitizeURL(u); result != u {
			t.Errorf("sanitizeURL(%q) = %q, want unchanged", u, result)
		}
	}
}

// --- Inline HTML rendering ---

func TestProcessInlineHTML_SanitizesLinkHref(t *testing.T) {
	input := `[click me](javascript:alert(1))`
	result := processInlineHTML(input)
	if strings.Contains(result, "javascript:") {
		t.Errorf("processInlineHTML should sanitize javascript: URLs, got: %s", result)
	}
	if !strings.Contains(result, `href="#"`) {
		t.Errorf("processInlineHTML should replace dangerous href with #, got: %s", result)
	}
}

func TestProcessInlineHTML_SanitizesImageSrc(t *testing.T) {
	input := `![xss](javascript:alert(1))`
	result := processInlineHTML(input)
	if strings.Contains(result, "javascript:") {
		t.Errorf("processInlineHTML should sanitize javascript: in image src, got: %s", result)
	}
}

func TestProcessInlineHTML_EscapesHTMLInText(t *testing.T) {
	input := `<script>alert(1)</script>`
	result := processInlineHTML(input)
	if strings.Contains(result, "<script>") {
		t.Errorf("processInlineHTML should escape HTML tags, got: %s", result)
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Errorf("processInlineHTML should produce escaped tags, got: %s", result)
	}
}

func TestProcessInlineHTML_PreservesNormalLinks(t *testing.T) {
	input := `[docs](https://example.com/docs)`
	result := processInlineHTML(input)
	if !strings.Contains(result, `href="https://example.com/docs"`) {
		t.Errorf("processInlineHTML should preserve https links, got: %s", result)
	}
	if !strings.Contains(result, ">docs<") {
		t.Errorf("processInlineHTML should render link text, got: %s", result)
	}
}

// --- Video block HTML ---

func TestVideoBlockHTML_EscapesTitle(t *testing.T) {
	block := Block{
		Name:        `<img src=x onerror=alert(1)>.mp4`,
		ContentType: BlockContentVideo,
		Data: &VideoData{
			Src:  "/video",
			MIME: "video/mp4",
		},
	}
	result := formatVideoBlockHTML(&block)
	if strings.Contains(result, `<img src=x`) {
		t.Errorf("formatVideoBlockHTML should escape title, got unescaped HTML in output")
	}
}

func TestVideoBlockHTML_EscapesMime(t *testing.T) {
	block := Block{
		Name:        "test.mp4",
		ContentType: BlockContentVideo,
		Data: &VideoData{
			Src:  "/video",
			MIME: `"><script>alert(1)</script>`,
		},
	}
	result := formatVideoBlockHTML(&block)
	if strings.Contains(result, "<script>alert") {
		t.Errorf("formatVideoBlockHTML should escape mime type")
	}
}

// --- Contract renderer ---

func TestContractPipeline_EscapesFrontmatter(t *testing.T) {
	content := "---\ntitle: <script>alert(\"xss\")</script>\ntype: contract\nparties: <img src=x onerror=alert(1)>\neffective: 2026-01-01\" onclick=\"alert(1)\n---\n## 1. Test\n\nBody text."
	parser := &ContractParser{}
	blocks := parser.Parse(content)
	if len(blocks) == 0 {
		t.Fatal("ContractParser should produce blocks")
	}
	result := RenderStaticHTMLPage("Test", blocks, false)

	if strings.Contains(result, "<script>alert") {
		t.Errorf("Contract pipeline should escape title")
	}
	if strings.Contains(result, `<img src=x`) {
		t.Errorf("Contract pipeline should escape parties")
	}
	if strings.Contains(result, `"alert(1)`) {
		t.Errorf("Contract pipeline should escape quotes in effective date")
	}
}

// --- Clause parsing ---

func TestParseContractClauses_Structure(t *testing.T) {
	content := `# Agreement

Preamble text here.

## 1. Definitions

Definition body.

## 2. Scope

Scope body.

### 2.1. Sub-scope

Sub-scope body.`

	preamble, clauses := parseContractClauses(content)

	if !strings.Contains(preamble, "Preamble text") {
		t.Errorf("expected preamble to contain 'Preamble text', got: %s", preamble)
	}
	if len(clauses) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(clauses))
	}
	if clauses[0].ID != "1" || clauses[0].Title != "Definitions" {
		t.Errorf("clause 0: expected '1. Definitions', got '%s. %s'", clauses[0].ID, clauses[0].Title)
	}
	if clauses[2].ID != "2.1" || clauses[2].Level != 2 {
		t.Errorf("clause 2: expected '2.1' level 2, got '%s' level %d", clauses[2].ID, clauses[2].Level)
	}
}


