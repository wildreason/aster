package main

import (
	"crypto/sha1"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RenderHTMLPage renders blocks as a full HTML document with enhanced web features
func RenderHTMLPage(title string, blocks []Block, showLineNums bool) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(title)))

	// highlight.js CDN for syntax highlighting (light theme)
	sb.WriteString("<link rel=\"stylesheet\" href=\"https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css\">\n")
	sb.WriteString("<script src=\"https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js\"></script>\n")

	sb.WriteString("<style>\n")
	sb.WriteString(cssStyles())
	sb.WriteString("</style>\n")
	sb.WriteString("</head>\n<body>\n")

	transcript := isTranscriptContent(blocks)

	if transcript {
		// Transcript mode: no TOC, sticky header, centered container
		sb.WriteString("<main class=\"transcript\">\n")
		sb.WriteString("<div class=\"transcript-header\">\n")
		sb.WriteString(fmt.Sprintf("<div class=\"transcript-title\">%s</div>\n", html.EscapeString(title)))
		sb.WriteString(fmt.Sprintf("<div class=\"transcript-meta\">%d turns</div>\n", len(blocks)))
		sb.WriteString("</div>\n")

		for i := range blocks {
			sb.WriteString(formatBlockHTML(&blocks[i], showLineNums, false))
		}

		sb.WriteString("</main>\n")

		sb.WriteString("<script>\n")
		sb.WriteString(enhancedScript())
		sb.WriteString("\nwindow.scrollTo(0, document.body.scrollHeight);\n")
		sb.WriteString("</script>\n")
	} else {
		// Standard mode: TOC, search, normal container
		headers := collectHeaders(blocks)

		// TOC sidebar
		if len(headers) > 1 {
			sb.WriteString("<nav id=\"toc\" class=\"toc\">\n")
			sb.WriteString("<div class=\"toc-toggle\" onclick=\"document.getElementById('toc').classList.toggle('collapsed')\" title=\"Toggle TOC\"><svg class=\"toc-icon\" width=\"16\" height=\"16\" viewBox=\"0 0 16 16\" fill=\"none\"><line x1=\"1\" y1=\"3\" x2=\"15\" y2=\"3\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/><line x1=\"1\" y1=\"8\" x2=\"11\" y2=\"8\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/><line x1=\"1\" y1=\"13\" x2=\"13\" y2=\"13\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/></svg><span class=\"toc-label\">Contents</span></div>\n")
			sb.WriteString("<div class=\"toc-content\">\n")
			sb.WriteString(fmt.Sprintf("<div class=\"toc-title\">%s</div>\n", html.EscapeString(title)))
			for _, h := range headers {
				class := "toc-h1"
				if h.level == 2 {
					class = "toc-h2"
				} else if h.level == 3 {
					class = "toc-h3"
				}
				sb.WriteString(fmt.Sprintf("<a class=\"toc-link %s\" href=\"#%s\" data-target=\"%s\">%s</a>\n",
					class, h.id, h.id, html.EscapeString(h.text)))
			}
			sb.WriteString("</div>\n</nav>\n")
		}

		// Main content
		containerClass := "container"
		if len(headers) > 1 {
			containerClass = "container has-toc"
		}
		sb.WriteString(fmt.Sprintf("<main class=\"%s\">\n", containerClass))

		singleBlock := len(blocks) == 1
		for i := range blocks {
			sb.WriteString(formatBlockHTML(&blocks[i], showLineNums, singleBlock))
		}

		sb.WriteString("</main>\n")

		// Search overlay
		sb.WriteString(searchOverlayHTML())

		sb.WriteString("<script>\n")
		sb.WriteString(enhancedScript())
		sb.WriteString("</script>\n")
	}

	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// DocMeta holds metadata for a document in directory mode
type DocMeta struct {
	Slug    string
	Title   string
	Created string
	Tags    []string
	ModTime time.Time
}

// RenderIndexPage renders a directory listing as an HTML page
func RenderIndexPage(dirName string, docs []DocMeta) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(dirName)))
	sb.WriteString("<style>\n")
	sb.WriteString(indexCSS())
	sb.WriteString("</style>\n")
	sb.WriteString("</head>\n<body>\n")

	sb.WriteString("<main class=\"index-container\">\n")
	sb.WriteString(fmt.Sprintf("<h1 class=\"index-title\">%s</h1>\n", html.EscapeString(dirName)))
	sb.WriteString(fmt.Sprintf("<p class=\"index-count\">%d documents</p>\n", len(docs)))

	if len(docs) == 0 {
		sb.WriteString("<p class=\"index-empty\">No markdown files found.</p>\n")
	} else {
		sb.WriteString("<table class=\"index-table\">\n")
		sb.WriteString("<thead><tr><th>Title</th><th>Created</th><th>Tags</th></tr></thead>\n")
		sb.WriteString("<tbody>\n")
		for _, doc := range docs {
			title := html.EscapeString(doc.Title)
			slug := html.EscapeString(doc.Slug)
			created := html.EscapeString(doc.Created)
			var tagParts []string
			for _, tag := range doc.Tags {
				tagParts = append(tagParts, fmt.Sprintf("<span class=\"tag\">%s</span>", html.EscapeString(tag)))
			}
			tags := strings.Join(tagParts, " ")
			sb.WriteString(fmt.Sprintf("<tr><td><a href=\"/%s\">%s</a></td><td class=\"date-col\">%s</td><td>%s</td></tr>\n",
				slug, title, created, tags))
		}
		sb.WriteString("</tbody>\n</table>\n")
	}

	sb.WriteString("</main>\n")

	// SSE live reload
	sb.WriteString("<script>\n")
	sb.WriteString("var es = new EventSource('/events');\n")
	sb.WriteString("es.onmessage = function(e) { if (e.data === 'reload') location.reload(); };\n")
	sb.WriteString("es.onerror = function() { setTimeout(function() { location.reload(); }, 2000); };\n")
	sb.WriteString("</script>\n")
	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// indexCSS returns CSS for the index page
func indexCSS() string {
	return `
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: #fff;
  color: #1d1d1f;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Display', 'Helvetica Neue', Helvetica, Arial, sans-serif;
  font-weight: 400;
  font-size: 17px;
  line-height: 1.47059;
  -webkit-font-smoothing: antialiased;
}
.index-container {
  max-width: 740px;
  margin: 0 auto;
  padding: 3rem 1.5rem;
}
.index-title {
  color: #1d1d1f;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.005em;
  margin-bottom: 0.25rem;
}
.index-count {
  color: #6e6e73;
  font-size: 14px;
  margin-bottom: 2rem;
}
.index-empty {
  color: #6e6e73;
  font-size: 14px;
}
.index-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
.index-table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  color: #6e6e73;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 2px solid #d2d2d7;
}
.index-table td {
  padding: 0.6rem 0.75rem;
  border-bottom: 1px solid #e8e8ed;
  vertical-align: top;
}
.index-table tr:hover td { background: #f5f5f7; }
.index-table a {
  color: #1d1d1f;
  text-decoration: none;
  font-weight: 600;
}
.index-table a:hover { color: #06c; }
.date-col {
  color: #6e6e73;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  font-size: 13px;
  white-space: nowrap;
}
.tag {
  display: inline-block;
  background: #e8e8ed;
  color: #1d1d1f;
  padding: 0.1rem 0.45rem;
  border-radius: 3px;
  font-size: 12px;
  margin-right: 0.3rem;
}
`
}

// tocHeader represents a header for the table of contents
type tocHeader struct {
	level int
	text  string
	id    string
}

// collectHeaders scans all blocks for h1/h2/h3 headers
func collectHeaders(blocks []Block) []tocHeader {
	var headers []tocHeader
	for _, block := range blocks {
		for _, page := range block.Pages {
			for _, line := range strings.Split(page, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "### ") {
					text := strings.TrimPrefix(trimmed, "### ")
					headers = append(headers, tocHeader{level: 3, text: text, id: headerID(text)})
				} else if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
					text := strings.TrimPrefix(trimmed, "## ")
					headers = append(headers, tocHeader{level: 2, text: text, id: headerID(text)})
				} else if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
					text := strings.TrimPrefix(trimmed, "# ")
					headers = append(headers, tocHeader{level: 1, text: text, id: headerID(text)})
				}
			}
		}
	}
	return headers
}

// headerID generates a URL-safe anchor ID from header text
func headerID(text string) string {
	// Strip markdown formatting
	text = regexp.MustCompile(`[*_` + "`" + `\[\]()]`).ReplaceAllString(text, "")
	text = strings.ToLower(strings.TrimSpace(text))
	text = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(text, "-")
	text = strings.Trim(text, "-")
	if text == "" {
		text = fmt.Sprintf("h-%x", sha1.Sum([]byte(text)))[:8]
	}
	return text
}

// isTranscriptContent checks if any block is a transcript
func isTranscriptContent(blocks []Block) bool {
	for _, b := range blocks {
		if b.ContentType == BlockContentTranscript {
			return true
		}
	}
	return false
}

// formatTranscriptBlockHTML renders a single conversation turn as HTML
func formatTranscriptBlockHTML(block *Block) string {
	var sb strings.Builder

	// Extract turn number from block name (e.g., "Turn 3" -> "3")
	turnLabel := block.Name
	if turnLabel == "" {
		turnLabel = "Turn"
	}

	sb.WriteString("<div class=\"turn\">\n")
	sb.WriteString(fmt.Sprintf("<div class=\"turn-gutter\">%s</div>\n", html.EscapeString(turnLabel)))

	var parts []TurnPart
	if tData, ok := block.Data.(*TranscriptData); ok {
		parts = tData.TurnParts
	}

	for _, part := range parts {
		switch part.Type {
		case "user":
			sb.WriteString("<div class=\"turn-user\"><pre>")
			sb.WriteString(html.EscapeString(part.Content))
			sb.WriteString("</pre></div>\n")

		case "assistant":
			sb.WriteString("<div class=\"turn-assistant\">")
			sb.WriteString(formatMarkdownHTML(part.Content, block, 0, false))
			sb.WriteString("</div>\n")

		case "diff":
			sb.WriteString("<div class=\"turn-diff\">")
			if part.Meta != "" {
				sb.WriteString(fmt.Sprintf("<div class=\"turn-diff-header\">%s</div>", html.EscapeString(part.Meta)))
			}
			sb.WriteString(formatDiffHTML(part.Content))
			sb.WriteString("</div>\n")

		case "tool_result":
			sb.WriteString("<details class=\"turn-tool\"><summary>Tool Output</summary><pre>")
			sb.WriteString(html.EscapeString(part.Content))
			sb.WriteString("</pre></details>\n")

		case "question":
			sb.WriteString("<div class=\"turn-question\"><pre>")
			sb.WriteString(html.EscapeString(part.Content))
			sb.WriteString("</pre></div>\n")

		default:
			// Unknown part type: render as plain preformatted text
			sb.WriteString("<div class=\"turn-assistant\"><pre>")
			sb.WriteString(html.EscapeString(part.Content))
			sb.WriteString("</pre></div>\n")
		}
	}

	sb.WriteString("</div>\n")
	return sb.String()
}

// formatBlockHTML renders a single block with all pages concatenated
// When singleBlock is true, the block header bar is hidden (headings are in the content)
func formatBlockHTML(block *Block, showLineNums bool, singleBlock bool) string {
	// Transcript blocks use dedicated renderer
	if block.ContentType == BlockContentTranscript {
		return formatTranscriptBlockHTML(block)
	}

	// Image blocks: dedicated renderer
	if block.ContentType == BlockContentImage {
		return formatImageBlockHTML(block)
	}

	// Video blocks: dedicated renderer
	if block.ContentType == BlockContentVideo {
		return formatVideoBlockHTML(block)
	}

	// Contract blocks: dedicated renderer
	if block.ContentType == BlockContentContract {
		return formatContractBlockHTML(block)
	}

	var sb strings.Builder

	sb.WriteString("<article class=\"block\">\n")

	// Block header (skip for single-block documents where headings render inline)
	if !singleBlock {
		displayName := html.EscapeString(block.Name)
		sb.WriteString(fmt.Sprintf("<header class=\"block-header\">%s</header>\n", displayName))
	}

	// Render all pages
	for pageNum := range block.Pages {
		pageContent := block.Pages[pageNum]

		pageType := block.ContentType
		if len(block.PageTypes) > pageNum {
			pageType = block.PageTypes[pageNum]
		}

		switch pageType {
		case BlockContentDiff:
			sb.WriteString(formatDiffHTML(pageContent))
		case BlockContentJSON:
			sb.WriteString(formatCodeBlockHTML(pageContent, "json"))
		case BlockContentYAML:
			sb.WriteString(formatCodeBlockHTML(pageContent, "yaml"))
		case BlockContentCSV:
			sb.WriteString(formatCsvHTML(block))
		default:
			sb.WriteString(formatMarkdownHTML(pageContent, block, pageNum, showLineNums))
		}
	}

	sb.WriteString("</article>\n")
	return sb.String()
}

// formatImageBlockHTML renders an image block as HTML
func formatImageBlockHTML(block *Block) string {
	imgData, ok := block.Data.(*ImageData)
	if !ok {
		return "<p>Error: missing image data</p>\n"
	}

	var sb strings.Builder
	sb.WriteString("<div class=\"content image-content\">\n")

	src := imgData.Src
	if !imgData.Inline {
		// Server mode: reference the asset route
		src = "/asset/" + html.EscapeString(filepath.Base(imgData.Alt))
	}

	sb.WriteString(fmt.Sprintf("<div class=\"title\" style=\"text-align:center;font-size:14px;font-weight:600;color:#6e6e73;margin-bottom:1.5rem\">%s</div>\n", html.EscapeString(imgData.Alt)))
	sb.WriteString("<div class=\"img-container\" style=\"text-align:center\">\n")
	sb.WriteString(fmt.Sprintf("  <img src=\"%s\" alt=\"%s\" style=\"max-width:100%%;max-height:85vh;border-radius:6px;border:1px solid #d2d2d7;cursor:pointer\" onclick=\"this.classList.toggle('expanded')\">\n",
		src, html.EscapeString(imgData.Alt)))
	sb.WriteString("</div>\n")
	sb.WriteString("</div>\n")
	return sb.String()
}

// formatVideoBlockHTML renders a video block as HTML
func formatVideoBlockHTML(block *Block) string {
	vidData, ok := block.Data.(*VideoData)
	if !ok {
		return "<p>Error: missing video data</p>\n"
	}

	var sb strings.Builder
	title := html.EscapeString(block.Name)

	src := vidData.Src
	if !vidData.Inline {
		// Server mode: reference the asset route
		src = "/asset/" + html.EscapeString(block.Name)
	}
	mime := html.EscapeString(vidData.MIME)

	sb.WriteString("<div class=\"content video-content\" style=\"display:flex;flex-direction:column;align-items:center\">\n")
	sb.WriteString(fmt.Sprintf("<div class=\"title\" style=\"font-size:14px;font-weight:600;color:#6e6e73;margin-bottom:1.5rem\">%s</div>\n", title))
	sb.WriteString("<div class=\"video-container\" style=\"max-width:960px;width:100%%\">\n")
	sb.WriteString(fmt.Sprintf("  <video id=\"player\" controls style=\"width:100%%;border-radius:6px;border:1px solid #d2d2d7;background:#1d1d1f;outline:none\">\n    <source src=\"%s\" type=\"%s\">\n  </video>\n", src, mime))

	// Speed controls
	sb.WriteString(`  <div class="controls" style="display:flex;align-items:center;gap:0.75rem;margin-top:0.75rem;font-size:13px;color:#6e6e73;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace">
    <span id="time-display">0:00 / 0:00</span>
    <span style="flex:1"></span>
    <button class="speed-btn" data-speed="0.5" style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:4px;padding:0.2rem 0.5rem;font-size:12px;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;color:#6e6e73;cursor:pointer">0.5x</button>
    <button class="speed-btn active" data-speed="1" style="background:#06c;border:1px solid #06c;border-radius:4px;padding:0.2rem 0.5rem;font-size:12px;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;color:#fff;cursor:pointer">1x</button>
    <button class="speed-btn" data-speed="1.5" style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:4px;padding:0.2rem 0.5rem;font-size:12px;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;color:#6e6e73;cursor:pointer">1.5x</button>
    <button class="speed-btn" data-speed="2" style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:4px;padding:0.2rem 0.5rem;font-size:12px;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;color:#6e6e73;cursor:pointer">2x</button>
  </div>
  <div style="margin-top:1rem;font-size:12px;color:#86868b">
    <kbd style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:3px;padding:0.1rem 0.35rem;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;font-size:11px">Space</kbd> play/pause
    <kbd style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:3px;padding:0.1rem 0.35rem;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;font-size:11px">F</kbd> fullscreen
    <kbd style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:3px;padding:0.1rem 0.35rem;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;font-size:11px">&#x2190;</kbd> -5s
    <kbd style="background:#f5f5f7;border:1px solid #d2d2d7;border-radius:3px;padding:0.1rem 0.35rem;font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;font-size:11px">&#x2192;</kbd> +5s
  </div>
`)
	sb.WriteString("</div>\n")
	sb.WriteString("</div>\n")
	return sb.String()
}

// formatContractBlockHTML renders a contract block as HTML
func formatContractBlockHTML(block *Block) string {
	cData, ok := block.Data.(*ContractData)
	if !ok {
		return "<p>Error: missing contract data</p>\n"
	}

	var sb strings.Builder

	// Contract header
	sb.WriteString("<div class=\"contract-header\" style=\"margin-bottom:2rem;padding-bottom:1rem;border-bottom:1px solid #d2d2d7\">\n")
	if cData.Parties != "" {
		sb.WriteString(fmt.Sprintf("<div style=\"font-size:13px;color:#6e6e73;font-weight:600;text-transform:uppercase;letter-spacing:0.03em\">%s</div>\n", html.EscapeString(cData.Parties)))
	}
	if cData.Effective != "" {
		sb.WriteString(fmt.Sprintf("<div style=\"font-size:13px;color:#86868b;margin-top:0.25rem\">Effective %s</div>\n", html.EscapeString(cData.Effective)))
	}
	sb.WriteString("</div>\n")

	// Preamble
	if cData.Preamble != "" {
		sb.WriteString("<div class=\"contract-preamble\" style=\"margin-bottom:2.5rem\">\n")
		sb.WriteString(renderPreambleHTML(cData.Preamble))
		sb.WriteString("</div>\n")
	}

	// Clauses
	for _, clause := range cData.Clauses {
		levelClass := "clause-l1"
		indent := ""
		if clause.Level == 2 {
			levelClass = "clause-l2"
			indent = " style=\"margin-left:1.5rem\""
		}

		sb.WriteString(fmt.Sprintf("<section class=\"clause %s\"%s id=\"clause-%s\">\n",
			levelClass, indent, html.EscapeString(clause.ID)))

		tag := "h2"
		if clause.Level == 2 {
			tag = "h3"
		}
		sb.WriteString(fmt.Sprintf("<%s style=\"font-size:20px;font-weight:600;color:#1d1d1f;margin-bottom:0.75rem\"><span style=\"font-family:'SF Mono',SFMono-Regular,ui-monospace,Menlo,monospace;font-size:0.85em;color:#6e6e73\">%s.</span> %s</%s>\n",
			tag, html.EscapeString(clause.ID), html.EscapeString(clause.Title), tag))

		sb.WriteString("<div class=\"clause-body\">\n")
		sb.WriteString(renderClauseBodyHTML(clause.Body))
		sb.WriteString("</div>\n")

		sb.WriteString("</section>\n")
	}

	return sb.String()
}

// formatCodeBlockHTML renders content as a single syntax-highlighted code block
func formatCodeBlockHTML(content string, lang string) string {
	var sb strings.Builder
	sb.WriteString("<div class=\"content\">\n")
	sb.WriteString(fmt.Sprintf("<div class=\"code-block\"><span class=\"code-lang\">%s</span>", html.EscapeString(lang)))
	sb.WriteString("<button class=\"copy-btn\" onclick=\"copyCode(this)\" title=\"Copy\">Copy</button>")
	sb.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">", html.EscapeString(lang)))
	sb.WriteString(html.EscapeString(content))
	sb.WriteString("</code></pre></div>\n")
	sb.WriteString("</div>\n")
	return sb.String()
}

// formatCsvHTML renders CSV data as an interactive table with filtering and optional chart
func formatCsvHTML(block *Block) string {
	var records [][]string
	if csvData, ok := block.Data.(*CsvData); ok {
		records = csvData.Records
	}
	if len(records) < 1 {
		return "<div class=\"content\"><p>Empty CSV</p></div>\n"
	}

	headers := records[0]
	dataRows := records[1:]

	var sb strings.Builder
	sb.WriteString("<div class=\"content csv-content\">\n")

	// Metadata header
	sb.WriteString(fmt.Sprintf("<div class=\"csv-meta\">%d rows x %d columns</div>\n",
		len(dataRows), len(headers)))

	// Auto-chart if data shape fits
	chart := csvAutoChart(headers, dataRows)
	if chart != "" {
		sb.WriteString("<div class=\"csv-chart\">\n")
		sb.WriteString(chart)
		sb.WriteString("</div>\n")
	}

	// Row count display
	sb.WriteString(fmt.Sprintf("<div class=\"csv-row-count\" id=\"csv-row-count\">Showing %d of %d rows</div>\n",
		len(dataRows), len(dataRows)))

	// Table with filter row
	sb.WriteString("<div class=\"table-scroll\">\n")
	sb.WriteString("<table class=\"sortable csv-table\" id=\"csv-table\">\n")

	// Thead: filter row + header row
	sb.WriteString("<thead>\n")
	// Filter row
	sb.WriteString("<tr class=\"filter-row\">")
	for colIdx := range headers {
		sb.WriteString(fmt.Sprintf("<th><input type=\"text\" class=\"col-filter\" data-col=\"%d\" placeholder=\"Filter...\" autocomplete=\"off\"></th>", colIdx))
	}
	sb.WriteString("</tr>\n")
	// Header row
	sb.WriteString("<tr>")
	for colIdx, h := range headers {
		sb.WriteString(fmt.Sprintf("<th onclick=\"sortTable(this, %d)\" class=\"sortable-th\">%s <span class=\"sort-icon\">&#x25B4;&#x25BE;</span></th>",
			colIdx, html.EscapeString(h)))
	}
	sb.WriteString("</tr>\n")
	sb.WriteString("</thead>\n")

	// Tbody
	sb.WriteString("<tbody>\n")
	for _, row := range dataRows {
		sb.WriteString("<tr>")
		for j := 0; j < len(headers); j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			// Right-align numeric cells
			if isNumericString(cell) {
				sb.WriteString(fmt.Sprintf("<td style=\"text-align:right\">%s</td>", html.EscapeString(cell)))
			} else {
				sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(cell)))
			}
		}
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</tbody>\n")
	sb.WriteString("</table>\n</div>\n")
	sb.WriteString("</div>\n")

	return sb.String()
}

// isNumericString checks if a string looks like a number
func isNumericString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	dotCount := 0
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
			continue
		}
		if c == ',' {
			continue // thousands separator
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// csvAutoChart detects if CSV data is chart-worthy and returns SVG
func csvAutoChart(headers []string, rows [][]string) string {
	if len(headers) < 2 || len(rows) < 2 {
		return ""
	}

	// Find numeric columns (skip first column as potential label/x-axis)
	numericCols := []int{}
	for col := 1; col < len(headers); col++ {
		isNumeric := true
		for _, row := range rows {
			if col >= len(row) || row[col] == "" {
				continue
			}
			if !isNumericString(row[col]) {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			numericCols = append(numericCols, col)
		}
	}

	if len(numericCols) == 0 {
		return ""
	}

	// Limit to first 2 numeric columns for readability
	if len(numericCols) > 2 {
		numericCols = numericCols[:2]
	}

	// Collect label + values
	type point struct {
		label string
		vals  []float64
	}

	var points []point
	for _, row := range rows {
		label := ""
		if len(row) > 0 {
			label = row[0]
		}
		vals := make([]float64, len(numericCols))
		for i, col := range numericCols {
			if col < len(row) {
				vals[i] = parseCSVFloat(row[col])
			}
		}
		points = append(points, point{label: label, vals: vals})
	}

	if len(points) < 2 {
		return ""
	}

	// Limit to 50 points for reasonable chart size
	if len(points) > 50 {
		points = points[:50]
	}

	// Find min/max across all series
	minVal := points[0].vals[0]
	maxVal := points[0].vals[0]
	for _, p := range points {
		for _, v := range p.vals {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}
	minVal -= valRange * 0.05
	maxVal += valRange * 0.05
	valRange = maxVal - minVal

	// SVG dimensions
	svgW := 700
	svgH := 280
	padL := 60
	padR := 20
	padT := 20
	padB := 60
	chartW := svgW - padL - padR
	chartH := svgH - padT - padB

	colors := []string{"#06c", "#1d1d1f"}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<svg viewBox=\"0 0 %d %d\" class=\"csv-svg\">\n", svgW, svgH))

	// Gridlines and Y-axis labels
	gridSteps := 5
	for i := 0; i <= gridSteps; i++ {
		y := padT + chartH - (i*chartH)/gridSteps
		val := minVal + (float64(i)/float64(gridSteps))*valRange
		sb.WriteString(fmt.Sprintf("<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#d2d2d7\" stroke-width=\"1\"/>\n",
			padL, y, padL+chartW, y))
		sb.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" text-anchor=\"end\" fill=\"#6e6e73\" font-size=\"11\" font-family=\"-apple-system,sans-serif\">%.0f</text>\n",
			padL-8, y+4, val))
	}

	// Axes
	sb.WriteString(fmt.Sprintf("<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#d2d2d7\" stroke-width=\"1\"/>\n",
		padL, padT+chartH, padL+chartW, padT+chartH))

	// Plot each series
	for si := range numericCols {
		color := colors[si%len(colors)]

		// Build polyline points
		var polyPoints []string
		for i, p := range points {
			x := padL + (i*chartW)/(len(points)-1)
			y := padT + chartH - int(((p.vals[si]-minVal)/valRange)*float64(chartH))
			polyPoints = append(polyPoints, fmt.Sprintf("%d,%d", x, y))
		}

		// Line
		sb.WriteString(fmt.Sprintf("<polyline points=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"2\"/>\n",
			strings.Join(polyPoints, " "), color))

		// Dots
		for i, p := range points {
			x := padL + (i*chartW)/(len(points)-1)
			y := padT + chartH - int(((p.vals[si]-minVal)/valRange)*float64(chartH))
			sb.WriteString(fmt.Sprintf("<circle cx=\"%d\" cy=\"%d\" r=\"3\" fill=\"%s\"/>\n", x, y, color))
			_ = p // used above
		}
	}

	// X-axis labels (show subset to avoid overlap)
	labelStep := 1
	if len(points) > 15 {
		labelStep = len(points) / 10
	}
	for i := 0; i < len(points); i += labelStep {
		x := padL + (i*chartW)/(len(points)-1)
		label := points[i].label
		if len(label) > 12 {
			label = label[:12]
		}
		sb.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" text-anchor=\"middle\" fill=\"#6e6e73\" font-size=\"11\" font-family=\"-apple-system,sans-serif\" transform=\"rotate(-45 %d %d)\">%s</text>\n",
			x, padT+chartH+16, x, padT+chartH+16, html.EscapeString(label)))
	}

	// Legend
	for si, col := range numericCols {
		color := colors[si%len(colors)]
		lx := padL + si*120
		sb.WriteString(fmt.Sprintf("<rect x=\"%d\" y=\"%d\" width=\"12\" height=\"12\" fill=\"%s\" rx=\"2\"/>\n",
			lx, svgH-16, color))
		sb.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" fill=\"#1d1d1f\" font-size=\"12\" font-family=\"-apple-system,sans-serif\">%s</text>\n",
			lx+16, svgH-5, html.EscapeString(headers[col])))
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// parseCSVFloat parses a string as float64, handling commas as thousands separators
func parseCSVFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

// formatMarkdownHTML renders markdown content as HTML
func formatMarkdownHTML(text string, block *Block, pageNum int, showLineNums bool) string {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	inCodeBlock := false
	var codeLines []string
	var codeLang string
	inTable := false
	var tableLines []string

	// Determine starting line number for this page
	startLine := 0
	if showLineNums && len(block.PageStartLine) > pageNum {
		startLine = block.PageStartLine[pageNum]
	}

	sb.WriteString("<div class=\"content\">\n")

	flushTable := func() {
		if len(tableLines) > 0 {
			sb.WriteString(renderTableHTML(tableLines))
			tableLines = nil
		}
		inTable = false
	}

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Table detection (before code blocks)
		if !inCodeBlock && isTableLine(trimmed) {
			if !inTable {
				inTable = true
				tableLines = []string{line}
			} else {
				tableLines = append(tableLines, line)
			}
			continue
		} else if inTable {
			flushTable()
		}

		// Code fence
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				codeLines = []string{}
				inCodeBlock = true
			} else {
				// End code block - with highlight.js class and copy button
				langClass := ""
				if codeLang != "" {
					langClass = fmt.Sprintf(" class=\"language-%s\"", html.EscapeString(codeLang))
				}
				langLabel := ""
				if codeLang != "" {
					langLabel = fmt.Sprintf("<span class=\"code-lang\">%s</span>", html.EscapeString(codeLang))
				}
				sb.WriteString(fmt.Sprintf("<div class=\"code-block\">%s<button class=\"copy-btn\" onclick=\"copyCode(this)\" title=\"Copy\">Copy</button><pre><code%s>", langLabel, langClass))
				for _, cl := range codeLines {
					sb.WriteString(html.EscapeString(cl))
					sb.WriteString("\n")
				}
				sb.WriteString("</code></pre></div>\n")
				inCodeBlock = false
				codeLines = nil
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Line number gutter
		lineNumHTML := ""
		if showLineNums && startLine > 0 {
			fileLineNum := startLine + lineIdx
			lineNumHTML = fmt.Sprintf("<span class=\"line-num\">%d</span>", fileLineNum)
		}

		// Headers with anchor IDs
		if strings.HasPrefix(trimmed, "### ") {
			raw := strings.TrimPrefix(trimmed, "### ")
			content := processInlineHTML(raw)
			id := headerID(raw)
			sb.WriteString(fmt.Sprintf("<h3 id=\"%s\">%s<a class=\"anchor\" href=\"#%s\">#</a>%s</h3>\n", id, lineNumHTML, id, content))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			raw := strings.TrimPrefix(trimmed, "## ")
			content := processInlineHTML(raw)
			id := headerID(raw)
			sb.WriteString(fmt.Sprintf("<h2 id=\"%s\">%s<a class=\"anchor\" href=\"#%s\">#</a>%s</h2>\n", id, lineNumHTML, id, content))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			raw := strings.TrimPrefix(trimmed, "# ")
			content := processInlineHTML(raw)
			id := headerID(raw)
			sb.WriteString(fmt.Sprintf("<h1 id=\"%s\">%s<a class=\"anchor\" href=\"#%s\">#</a>%s</h1>\n", id, lineNumHTML, id, content))
			continue
		}

		// Horizontal rule
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			sb.WriteString("<hr>\n")
			continue
		}

		// Empty line
		if trimmed == "" {
			sb.WriteString("<br>\n")
			continue
		}

		// Image: ![alt](url)
		if imgMatch := regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)$`).FindStringSubmatch(trimmed); imgMatch != nil {
			alt := html.EscapeString(imgMatch[1])
			src := html.EscapeString(imgMatch[2])
			sb.WriteString(fmt.Sprintf("<div class=\"img-wrapper\">%s<img src=\"%s\" alt=\"%s\" loading=\"lazy\" onclick=\"this.classList.toggle('expanded')\"><div class=\"img-caption\">%s</div></div>\n", lineNumHTML, src, alt, alt))
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			leadingSpaces := len(line) - len(strings.TrimLeft(line, " \t"))
			content := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			content = processInlineHTML(content)
			class := "list-item"
			if leadingSpaces >= 2 {
				class = "list-item nested"
			}
			sb.WriteString(fmt.Sprintf("<div class=\"%s\">%s<span class=\"bullet\">-</span> %s</div>\n", class, lineNumHTML, content))
			continue
		}

		// Numbered list
		if numMatch := regexp.MustCompile(`^(\d+)\.\s+(.+)$`).FindStringSubmatch(trimmed); numMatch != nil {
			content := processInlineHTML(numMatch[2])
			sb.WriteString(fmt.Sprintf("<div class=\"list-item\">%s<span class=\"list-num\">%s.</span> %s</div>\n", lineNumHTML, html.EscapeString(numMatch[1]), content))
			continue
		}

		// Regular paragraph
		content := processInlineHTML(trimmed)
		sb.WriteString(fmt.Sprintf("<p>%s%s</p>\n", lineNumHTML, content))
	}

	// Flush unclosed table
	if inTable {
		flushTable()
	}

	// Flush unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		sb.WriteString("<div class=\"code-block\"><pre><code>")
		for _, cl := range codeLines {
			sb.WriteString(html.EscapeString(cl))
			sb.WriteString("\n")
		}
		sb.WriteString("</code></pre></div>\n")
	}

	sb.WriteString("</div>\n")
	return sb.String()
}

// sanitizeURL strips dangerous URL schemes (javascript:, data:, vbscript:)
func sanitizeURL(u string) string {
	lower := strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
		return "#"
	}
	return u
}

// processInlineHTML handles inline markdown: bold, italic, code, links, images
func processInlineHTML(text string) string {
	escaped := html.EscapeString(text)

	// Inline code: `code`
	codeRe := regexp.MustCompile("`([^`]+)`")
	escaped = codeRe.ReplaceAllString(escaped, "<code class=\"inline\">$1</code>")

	// Inline images: ![alt](url)
	imgRe := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	escaped = imgRe.ReplaceAllStringFunc(escaped, func(m string) string {
		parts := imgRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		return fmt.Sprintf(`<img class="inline-img" src="%s" alt="%s" loading="lazy">`, sanitizeURL(parts[2]), parts[1])
	})

	// Bold: **text**
	boldRe := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	escaped = boldRe.ReplaceAllString(escaped, "<strong>$1</strong>")

	// Bold: __text__
	boldUnderRe := regexp.MustCompile(`__([^_]+)__`)
	escaped = boldUnderRe.ReplaceAllString(escaped, "<strong>$1</strong>")

	// Strikethrough: ~~text~~
	strikeRe := regexp.MustCompile(`~~([^~]+)~~`)
	escaped = strikeRe.ReplaceAllString(escaped, "<del>$1</del>")

	// Italic: *text* (not **)
	italicRe := regexp.MustCompile(`\*([^*]+)\*`)
	escaped = italicRe.ReplaceAllString(escaped, "<em>$1</em>")

	// Links: [text](url) -- open in new tab with external icon
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	escaped = linkRe.ReplaceAllStringFunc(escaped, func(m string) string {
		parts := linkRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		href := sanitizeURL(parts[2])
		return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener" title="%s">%s<span class="ext-icon">&#x2197;</span></a>`, href, href, parts[1])
	})

	return escaped
}

// formatDiffHTML renders diff content with side-by-side view, collapsible hunks, and word-level highlighting
func formatDiffHTML(content string) string {
	hunks := ParseHunks(content)
	if len(hunks) == 0 {
		return "<pre>" + html.EscapeString(content) + "</pre>\n"
	}

	var sb strings.Builder
	sb.WriteString("<div class=\"diff\">\n")

	for hunkIdx, hunk := range hunks {
		hunkID := fmt.Sprintf("hunk-%d", hunkIdx)
		sb.WriteString(fmt.Sprintf("<div class=\"diff-hunk\" id=\"%s\">\n", hunkID))
		sb.WriteString(fmt.Sprintf("<div class=\"diff-hunk-header\" onclick=\"toggleHunk('%s-body')\">", hunkID))
		sb.WriteString(fmt.Sprintf("<span class=\"diff-hunk-toggle\">&#x25BC;</span> Hunk %d", hunkIdx+1))
		if hunk.Header != "" {
			sb.WriteString(fmt.Sprintf(" <span class=\"diff-hunk-range\">%s</span>", html.EscapeString(hunk.Header)))
		}
		sb.WriteString("</div>\n")
		sb.WriteString(fmt.Sprintf("<div class=\"diff-hunk-body\" id=\"%s-body\">\n", hunkID))

		// Build paired lines for side-by-side
		sb.WriteString("<table class=\"diff-table\"><colgroup><col class=\"diff-col-num\"><col class=\"diff-col-content\"><col class=\"diff-col-num\"><col class=\"diff-col-content\"></colgroup>\n")

		oldLineNum := hunk.StartOld
		newLineNum := hunk.StartNew

		// Group consecutive removed+added for word-level diff
		i := 0
		for i < len(hunk.Lines) {
			line := hunk.Lines[i]

			if line.Type == DiffContext {
				sb.WriteString(fmt.Sprintf("<tr class=\"diff-row-context\"><td class=\"diff-num\">%d</td><td class=\"diff-code\"> %s</td><td class=\"diff-num\">%d</td><td class=\"diff-code\"> %s</td></tr>\n",
					oldLineNum, html.EscapeString(line.Content), newLineNum, html.EscapeString(line.Content)))
				oldLineNum++
				newLineNum++
				i++
				continue
			}

			// Collect consecutive removed lines
			var removed []DiffLine
			for i < len(hunk.Lines) && hunk.Lines[i].Type == DiffRemoved {
				removed = append(removed, hunk.Lines[i])
				i++
			}
			// Collect consecutive added lines
			var added []DiffLine
			for i < len(hunk.Lines) && hunk.Lines[i].Type == DiffAdded {
				added = append(added, hunk.Lines[i])
				i++
			}

			// Pair them up for side-by-side with word-level diff
			maxPairs := len(removed)
			if len(added) > maxPairs {
				maxPairs = len(added)
			}

			for j := 0; j < maxPairs; j++ {
				leftNum := ""
				leftContent := ""
				leftClass := "diff-cell-empty"
				rightNum := ""
				rightContent := ""
				rightClass := "diff-cell-empty"

				if j < len(removed) {
					leftNum = fmt.Sprintf("%d", oldLineNum)
					leftClass = "diff-cell-removed"
					oldLineNum++

					if j < len(added) {
						// Word-level diff between paired lines
						leftHL, rightHL := wordDiffHTML(removed[j].Content, added[j].Content)
						leftContent = leftHL
						rightNum = fmt.Sprintf("%d", newLineNum)
						rightClass = "diff-cell-added"
						rightContent = rightHL
						newLineNum++
					} else {
						leftContent = html.EscapeString(removed[j].Content)
					}
				} else if j < len(added) {
					rightNum = fmt.Sprintf("%d", newLineNum)
					rightClass = "diff-cell-added"
					rightContent = html.EscapeString(added[j].Content)
					newLineNum++
				}

				sb.WriteString(fmt.Sprintf("<tr><td class=\"diff-num %s\">%s</td><td class=\"diff-code %s\">%s</td><td class=\"diff-num %s\">%s</td><td class=\"diff-code %s\">%s</td></tr>\n",
					leftClass, leftNum, leftClass, leftContent,
					rightClass, rightNum, rightClass, rightContent))
			}
		}

		sb.WriteString("</table>\n")
		sb.WriteString("</div>\n</div>\n")
	}

	sb.WriteString("</div>\n")
	return sb.String()
}

// wordDiffHTML computes word-level diff between two lines and returns HTML with highlighted changes
func wordDiffHTML(oldLine, newLine string) (string, string) {
	oldWords := strings.Fields(oldLine)
	newWords := strings.Fields(newLine)

	// Simple LCS-based word diff
	// Build match table
	m := len(oldWords)
	n := len(newWords)

	// LCS length table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldWords[i-1] == newWords[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to find which words match
	oldMatched := make([]bool, m)
	newMatched := make([]bool, n)
	i, j := m, n
	for i > 0 && j > 0 {
		if oldWords[i-1] == newWords[j-1] {
			oldMatched[i-1] = true
			newMatched[j-1] = true
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Build HTML with highlights on non-matched words
	var oldHTML, newHTML strings.Builder
	for idx, w := range oldWords {
		if idx > 0 {
			oldHTML.WriteString(" ")
		}
		if oldMatched[idx] {
			oldHTML.WriteString(html.EscapeString(w))
		} else {
			oldHTML.WriteString("<span class=\"diff-word-del\">")
			oldHTML.WriteString(html.EscapeString(w))
			oldHTML.WriteString("</span>")
		}
	}
	for idx, w := range newWords {
		if idx > 0 {
			newHTML.WriteString(" ")
		}
		if newMatched[idx] {
			newHTML.WriteString(html.EscapeString(w))
		} else {
			newHTML.WriteString("<span class=\"diff-word-add\">")
			newHTML.WriteString(html.EscapeString(w))
			newHTML.WriteString("</span>")
		}
	}

	return oldHTML.String(), newHTML.String()
}

// renderTableHTML renders markdown table lines as a sortable HTML table with scroll wrapper
func renderTableHTML(lines []string) string {
	if len(lines) < 2 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<div class=\"table-scroll\">\n")
	sb.WriteString("<table class=\"sortable\">\n")

	headerCells := parseTableCells(lines[0])
	sb.WriteString("<thead><tr>")
	for colIdx, cell := range headerCells {
		sb.WriteString(fmt.Sprintf("<th onclick=\"sortTable(this, %d)\" class=\"sortable-th\">%s <span class=\"sort-icon\">&#x25B4;&#x25BE;</span></th>", colIdx, html.EscapeString(cell)))
	}
	sb.WriteString("</tr></thead>\n")

	sb.WriteString("<tbody>\n")
	for i := 1; i < len(lines); i++ {
		if isTableSeparator(lines[i]) {
			continue
		}
		cells := parseTableCells(lines[i])
		sb.WriteString("<tr>")
		for _, cell := range cells {
			sb.WriteString(fmt.Sprintf("<td>%s</td>", processInlineHTML(cell)))
		}
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</tbody>\n</table>\n</div>\n")

	return sb.String()
}

// searchOverlayHTML returns the search overlay markup
func searchOverlayHTML() string {
	return `<div id="search-overlay" class="search-overlay hidden">
<div class="search-box">
<input id="search-input" type="text" placeholder="Search..." autocomplete="off">
<div class="search-meta"><span id="search-count"></span><span class="search-hint">Esc to close / Enter to navigate</span></div>
</div>
<div id="search-results" class="search-results"></div>
</div>
`
}

// cssStyles returns the minimal Apple-style CSS
// Palette: Text #1d1d1f, Secondary #6e6e73, Tertiary #86868b,
//          Surface #f5f5f7, White #fff, Border #d2d2d7, Light border #e8e8ed,
//          Link #06c, Highlight #ffd60a
// Type: System font stack (body), SF Mono (code)
// Scale: H1 40px, H2 32px, H3 24px, Body 17px, Small 14px, Caption 12px
func cssStyles() string {
	return `
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  background: #fff;
  color: #1d1d1f;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Display', 'Helvetica Neue', Helvetica, Arial, sans-serif;
  font-weight: 400;
  font-size: 17px;
  line-height: 1.47059;
  -webkit-font-smoothing: antialiased;
}

/* --- Layout with TOC --- */
.container {
  max-width: 740px;
  margin: 0 auto;
  padding: 3rem 1.5rem;
}
.container.has-toc {
  max-width: 740px;
  padding: 3rem 2rem;
  margin: 0 auto;
  transform: translateX(140px);
  transition: transform 0.2s;
}

/* --- TOC sidebar --- */
.toc {
  position: fixed;
  top: 0;
  left: 0;
  width: 280px;
  height: 100vh;
  background: #fff;
  border-right: none;
  overflow-y: auto;
  padding: 2.5rem 0;
  z-index: 100;
  transition: transform 0.2s;
}
.toc.collapsed .toc-content { display: none; }
.toc.collapsed { width: 40px; }
.toc.collapsed ~ main.has-toc { transform: none; }

.toc-toggle {
  padding: 0.5rem 1.5rem 1.25rem;
  cursor: pointer;
  color: #1d1d1f;
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.toc-toggle:hover { color: #1d1d1f; }
.toc-icon { flex-shrink: 0; }
.toc.collapsed .toc-label { display: none; }
.toc.collapsed .toc-toggle { justify-content: center; padding-left: 0; padding-right: 0; }

.toc-title {
  display: none;
}

.toc-link {
  display: block;
  padding: 0.4rem 1.5rem;
  color: #86868b;
  text-decoration: none;
  font-size: 15px;
  border-left: 3px solid transparent;
  transition: all 0.15s;
  line-height: 1.4;
}
.toc-link:hover { color: #1d1d1f; }
.toc-link.active { color: #1d1d1f; font-weight: 600; border-left-color: #1d1d1f; }
.toc-h2 { padding-left: 2rem; font-size: 15px; }
.toc-h3 { padding-left: 2.5rem; font-size: 14px; }

@media (max-width: 1100px) {
  .toc { transform: translateX(-100%); }
  .toc.open { transform: translateX(0); }
  .toc-toggle { position: fixed; top: 0.5rem; left: 0.5rem; z-index: 101; background: #fff; border: 1px solid #d2d2d7; border-radius: 8px; padding: 0.4rem 0.75rem; font-size: 13px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  .container.has-toc { margin-left: auto; }
}

/* --- Blocks --- */
.block { margin-bottom: 3rem; }

.block-header {
  background: #f5f5f7;
  color: #1d1d1f;
  padding: 0.4rem 0.8rem;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  border-bottom: 1px solid #d2d2d7;
}

.content { padding: 0.5rem 0; }

/* --- Headers --- */
h1, h2, h3 { position: relative; font-weight: 700; color: #1d1d1f; letter-spacing: -0.005em; }
h1 { font-size: 40px; line-height: 1.1; margin: 2.5rem 0 0.25rem; padding-left: 0; }
h2 { font-size: 32px; line-height: 1.125; margin: 2.5rem 0 0.15rem; padding-left: 0; }
h3 { font-size: 24px; line-height: 1.16667; margin: 2rem 0 0.1rem; padding-left: 0; }

.anchor {
  position: absolute;
  left: -1.2em;
  color: #d2d2d7;
  text-decoration: none;
  font-size: 0.6em;
  top: 0.35em;
  opacity: 0;
  transition: opacity 0.15s;
}
h1:hover .anchor, h2:hover .anchor, h3:hover .anchor { opacity: 1; }
.anchor:hover { color: #06c; }

p { margin: 1em 0; }

strong { color: #1d1d1f; font-weight: 600; }
em { font-weight: 600; font-style: normal; }

/* --- Links --- */
a { color: #06c; text-decoration: none; position: relative; }
a:hover { text-decoration: underline; }
a[target="_blank"] .ext-icon {
  font-size: 0.65em;
  margin-left: 0.15em;
  opacity: 0.4;
  vertical-align: super;
}
a[target="_blank"]:hover .ext-icon { opacity: 0.8; }
a[target="_blank"]:hover::after {
  content: attr(title);
  position: absolute;
  bottom: 100%;
  left: 0;
  background: #1d1d1f;
  color: #fff;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 12px;
  white-space: nowrap;
  max-width: 400px;
  overflow: hidden;
  text-overflow: ellipsis;
  z-index: 50;
  pointer-events: none;
  box-shadow: 0 2px 8px rgba(0,0,0,0.12);
}

code.inline {
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  color: #1d1d1f;
  background: #f5f5f7;
  border: none;
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  font-size: 0.875em;
}

/* --- Code blocks --- */
.code-block {
  margin: 1.5rem 0;
  border: none;
  border-radius: 12px;
  overflow-x: auto;
  position: relative;
  background: #f5f5f7;
}

.code-block .code-lang {
  display: none;
}

.copy-btn {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  background: #fff;
  color: #1d1d1f;
  border: none;
  border-radius: 6px;
  padding: 0.35rem 0.75rem;
  font-size: 13px;
  font-weight: 500;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
  z-index: 2;
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
}
.code-block:hover .copy-btn { opacity: 1; }
.copy-btn:hover { background: #e8e8ed; }
.copy-btn.copied { color: #06c; }

.code-block pre {
  margin: 0;
  padding: 1.25rem 1.5rem;
  overflow-x: auto;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
}

.code-block code {
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  font-size: 14px;
}
.code-block pre code.hljs { background: transparent; padding: 0; }

/* --- Diff --- */
.diff { margin: 0.75rem 0; font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace; font-size: 14px; }

.diff-hunk { margin-bottom: 0.75rem; border: 1px solid #d2d2d7; border-radius: 6px; overflow: hidden; }

.diff-hunk-header {
  background: #f5f5f7;
  padding: 0.35rem 0.75rem;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif;
  color: #6e6e73;
  border-bottom: 1px solid #d2d2d7;
}
.diff-hunk-header:hover { color: #1d1d1f; }
.diff-hunk-toggle { display: inline-block; transition: transform 0.15s; font-size: 10px; margin-right: 0.3rem; }
.diff-hunk.collapsed .diff-hunk-toggle { transform: rotate(-90deg); }
.diff-hunk.collapsed .diff-hunk-body { display: none; }
.diff-hunk-range { color: #6e6e73; font-size: 11px; }

.diff-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}
.diff-col-num { width: 3.5em; }
.diff-col-content { width: calc(50% - 3.5em); }

.diff-table tr { border-bottom: none; }
.diff-num {
  color: #6e6e73;
  text-align: right;
  padding: 0 0.4rem;
  font-size: 12px;
  user-select: none;
  vertical-align: top;
  background: #f5f5f7;
}
.diff-code {
  padding: 0 0.5rem;
  white-space: pre;
  overflow-x: auto;
  vertical-align: top;
}

.diff-cell-removed { background: #FEF2F2; }
.diff-cell-added { background: #F0FDF4; }
.diff-cell-empty { background: #fff; }
.diff-row-context td { background: transparent; }
.diff-row-context .diff-code { color: #6e6e73; }

.diff-cell-removed .diff-num { background: #FEE2E2; color: #EF4444; }
.diff-cell-added .diff-num { background: #DCFCE7; color: #10B981; }

.diff-word-del { background: #FECACA; color: #991B1B; border-radius: 2px; padding: 0 1px; }
.diff-word-add { background: #BBF7D0; color: #166534; border-radius: 2px; padding: 0 1px; }

/* --- Tables --- */
.table-scroll {
  overflow-x: auto;
  margin: 1.5rem 0;
  border: 1px solid #d2d2d7;
  border-radius: 6px;
}

table {
  border-collapse: collapse;
  font-size: 14px;
  min-width: 100%;
}
th, td {
  border: 1px solid #d2d2d7;
  padding: 0.4rem 0.75rem;
  text-align: left;
}
th {
  background: #f5f5f7;
  color: #1d1d1f;
  font-weight: 600;
  font-size: 13px;
}
.sortable-th {
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
}
.sortable-th:hover { background: #e8e8ed; }
.sort-icon { font-size: 0.7em; color: #d2d2d7; margin-left: 0.3em; }
.sortable-th.asc .sort-icon { color: #06c; }
.sortable-th.desc .sort-icon { color: #06c; }

/* --- Images --- */
.img-wrapper {
  margin: 1.5rem 0;
}
.img-wrapper img {
  max-width: 100%;
  border-radius: 6px;
  border: 1px solid #d2d2d7;
  cursor: pointer;
  transition: max-width 0.2s;
}
.img-wrapper img.expanded { max-width: none; }
.img-caption {
  color: #6e6e73;
  font-size: 12px;
  margin-top: 0.3rem;
}
.inline-img {
  max-height: 1.4em;
  vertical-align: middle;
  border-radius: 2px;
}

/* --- Lists --- */
.list-item {
  padding-left: 1.5rem;
  margin: 0.5em 0;
}
.list-item.nested { padding-left: 3rem; }
.bullet { color: #1d1d1f; }
.list-num { color: #1d1d1f; font-weight: 600; }

/* --- Line numbers --- */
.line-num {
  color: #6e6e73;
  display: inline-block;
  min-width: 3em;
  text-align: right;
  padding-right: 0.8em;
  user-select: none;
  font-size: 12px;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
}

hr {
  border: none;
  border-top: 1px solid #d2d2d7;
  margin: 2rem 0;
}

br { display: block; content: ""; margin: 0.3rem 0; }

/* --- Search overlay --- */
.search-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0,0,0,0.1);
  z-index: 200;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 15vh;
}
.search-overlay.hidden { display: none; }

.search-box {
  background: #fff;
  border: 1px solid #d2d2d7;
  border-radius: 8px;
  width: 560px;
  max-width: 90vw;
  padding: 0.75rem 1rem;
  box-shadow: 0 4px 24px rgba(0,0,0,0.08);
}
.search-box input {
  width: 100%;
  background: transparent;
  border: none;
  color: #1d1d1f;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif;
  font-size: 17px;
  outline: none;
}
.search-box input::placeholder { color: #86868b; }
.search-meta {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #6e6e73;
  margin-top: 0.3rem;
}

.search-results {
  background: #fff;
  border: 1px solid #d2d2d7;
  border-radius: 8px;
  width: 560px;
  max-width: 90vw;
  max-height: 50vh;
  overflow-y: auto;
  margin-top: 0.3rem;
  box-shadow: 0 4px 24px rgba(0,0,0,0.08);
}
.search-results:empty { display: none; }

.search-result {
  padding: 0.5rem 1rem;
  cursor: pointer;
  border-bottom: 1px solid #e8e8ed;
  font-size: 14px;
}
.search-result:hover, .search-result.active { background: #f5f5f7; }
.search-result .sr-context { color: #6e6e73; font-size: 12px; }
.search-result mark { background: #ffd60a; color: #1d1d1f; border-radius: 2px; padding: 0 2px; }

/* Highlight in page */
.search-highlight { background: #ffd60a; border-radius: 2px; }

/* --- CSV --- */
.csv-meta {
  color: #6e6e73;
  font-size: 13px;
  margin-bottom: 0.5rem;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
}
.csv-row-count {
  color: #6e6e73;
  font-size: 12px;
  margin-bottom: 0.5rem;
}
.csv-chart {
  margin: 1rem 0;
  border: 1px solid #d2d2d7;
  border-radius: 6px;
  padding: 1rem;
  background: #fff;
}
.csv-svg {
  width: 100%;
  height: auto;
  max-height: 300px;
}
.csv-table .filter-row th {
  padding: 0.3rem 0.4rem;
  background: #fff;
  border-bottom: 1px solid #d2d2d7;
}
.col-filter {
  width: 100%;
  padding: 0.25rem 0.4rem;
  font-size: 12px;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif;
  border: 1px solid #d2d2d7;
  border-radius: 3px;
  background: #f5f5f7;
  color: #1d1d1f;
  outline: none;
  box-sizing: border-box;
}
.col-filter:focus { border-color: #06c; }

/* --- Transcript --- */
.transcript {
  max-width: 900px;
  margin: 0 auto;
  padding: 1rem 1.5rem 3rem;
}
.transcript-header {
  position: sticky;
  top: 0;
  background: #fff;
  border-bottom: 1px solid #d2d2d7;
  padding: 0.75rem 0;
  margin-bottom: 1.5rem;
  z-index: 50;
  display: flex;
  align-items: baseline;
  gap: 1rem;
}
.transcript-title {
  font-size: 14px;
  font-weight: 600;
  color: #1d1d1f;
}
.transcript-meta {
  font-size: 12px;
  color: #6e6e73;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
}
.turn {
  margin-bottom: 1.5rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid #e8e8ed;
}
.turn:last-child { border-bottom: none; }
.turn-gutter {
  color: #6e6e73;
  font-size: 12px;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  margin-bottom: 0.5rem;
}
.turn-user {
  background: #f5f5f7;
  color: #1d1d1f;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  margin-bottom: 0.75rem;
}
.turn-user pre {
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  font-size: 14px;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  color: inherit;
  background: transparent;
}
.turn-assistant {
  padding: 0.5rem 0;
}
.turn-assistant .content { padding: 0; }
.turn-diff {
  margin: 0.75rem 0;
}
.turn-diff-header {
  font-size: 12px;
  color: #6e6e73;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  margin-bottom: 0.25rem;
}
.turn-tool {
  margin: 0.5rem 0;
  border-left: 3px solid #d2d2d7;
  padding-left: 0.75rem;
}
.turn-tool summary {
  font-size: 13px;
  color: #6e6e73;
  cursor: pointer;
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  padding: 0.25rem 0;
}
.turn-tool summary:hover { color: #1d1d1f; }
.turn-tool pre {
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
  color: #1d1d1f;
  background: #f5f5f7;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  margin-top: 0.25rem;
  max-height: 300px;
  overflow-y: auto;
}
.turn-question {
  background: #f5f5f7;
  border: 1px solid #d2d2d7;
  border-radius: 6px;
  padding: 0.75rem 1rem;
  margin: 0.5rem 0;
}
.turn-question pre {
  font-family: 'SF Mono', SFMono-Regular, ui-monospace, Menlo, monospace;
  font-size: 14px;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  color: #1d1d1f;
  background: transparent;
}
`
}

// RenderStaticHTMLPage renders blocks as a self-contained HTML document (no CDN, no SSE)
func RenderStaticHTMLPage(title string, blocks []Block, showLineNums bool) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(title)))

	// Inline highlight.js CSS (no CDN)
	sb.WriteString("<style>\n")
	sb.WriteString(highlightCSS)
	sb.WriteString("\n</style>\n")

	// Inline highlight.js JS (no CDN)
	sb.WriteString("<script>\n")
	sb.WriteString(highlightJS)
	sb.WriteString("\n</script>\n")

	sb.WriteString("<style>\n")
	sb.WriteString(cssStyles())
	sb.WriteString("</style>\n")
	sb.WriteString("</head>\n<body>\n")

	transcript := isTranscriptContent(blocks)

	if transcript {
		// Transcript mode: no TOC, sticky header, centered container
		sb.WriteString("<main class=\"transcript\">\n")
		sb.WriteString("<div class=\"transcript-header\">\n")
		sb.WriteString(fmt.Sprintf("<div class=\"transcript-title\">%s</div>\n", html.EscapeString(title)))
		sb.WriteString(fmt.Sprintf("<div class=\"transcript-meta\">%d turns</div>\n", len(blocks)))
		sb.WriteString("</div>\n")

		for i := range blocks {
			sb.WriteString(formatBlockHTML(&blocks[i], showLineNums, false))
		}

		sb.WriteString("</main>\n")

		sb.WriteString("<script>\n")
		sb.WriteString(staticScript())
		sb.WriteString("\nwindow.scrollTo(0, document.body.scrollHeight);\n")
		sb.WriteString("</script>\n")
	} else {
		// Standard mode: TOC, search, normal container
		headers := collectHeaders(blocks)

		// TOC sidebar
		if len(headers) > 1 {
			sb.WriteString("<nav id=\"toc\" class=\"toc\">\n")
			sb.WriteString("<div class=\"toc-toggle\" onclick=\"document.getElementById('toc').classList.toggle('collapsed')\" title=\"Toggle TOC\"><svg class=\"toc-icon\" width=\"16\" height=\"16\" viewBox=\"0 0 16 16\" fill=\"none\"><line x1=\"1\" y1=\"3\" x2=\"15\" y2=\"3\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/><line x1=\"1\" y1=\"8\" x2=\"11\" y2=\"8\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/><line x1=\"1\" y1=\"13\" x2=\"13\" y2=\"13\" stroke=\"currentColor\" stroke-width=\"1.5\" stroke-linecap=\"round\"/></svg><span class=\"toc-label\">Contents</span></div>\n")
			sb.WriteString("<div class=\"toc-content\">\n")
			sb.WriteString(fmt.Sprintf("<div class=\"toc-title\">%s</div>\n", html.EscapeString(title)))
			for _, h := range headers {
				class := "toc-h1"
				if h.level == 2 {
					class = "toc-h2"
				} else if h.level == 3 {
					class = "toc-h3"
				}
				sb.WriteString(fmt.Sprintf("<a class=\"toc-link %s\" href=\"#%s\" data-target=\"%s\">%s</a>\n",
					class, h.id, h.id, html.EscapeString(h.text)))
			}
			sb.WriteString("</div>\n</nav>\n")
		}

		// Main content
		containerClass := "container"
		if len(headers) > 1 {
			containerClass = "container has-toc"
		}
		sb.WriteString(fmt.Sprintf("<main class=\"%s\">\n", containerClass))

		singleBlock := len(blocks) == 1
		for i := range blocks {
			sb.WriteString(formatBlockHTML(&blocks[i], showLineNums, singleBlock))
		}

		sb.WriteString("</main>\n")

		// Search overlay
		sb.WriteString(searchOverlayHTML())

		sb.WriteString("<script>\n")
		sb.WriteString(staticScript())
		sb.WriteString("</script>\n")
	}

	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// staticScript returns JavaScript for static export (no SSE live reload)
func staticScript() string {
	return `
/* --- Syntax highlighting --- */
document.querySelectorAll('.code-block pre code').forEach(function(el) {
  hljs.highlightElement(el);
});

/* --- Copy button --- */
function copyCode(btn) {
  var code = btn.parentElement.querySelector('code');
  var text = code.textContent || code.innerText;
  navigator.clipboard.writeText(text).then(function() {
    btn.classList.add('copied');
    btn.textContent = 'Copied';
    setTimeout(function() {
      btn.classList.remove('copied');
      btn.textContent = 'Copy';
    }, 1500);
  });
}

/* --- Table sorting --- */
function sortTable(th, colIdx) {
  var table = th.closest('table');
  var tbody = table.querySelector('tbody');
  var rows = Array.from(tbody.querySelectorAll('tr'));
  var isAsc = th.classList.contains('asc');

  table.querySelectorAll('.sortable-th').forEach(function(h) { h.classList.remove('asc', 'desc'); });

  if (isAsc) {
    th.classList.add('desc');
  } else {
    th.classList.add('asc');
  }

  rows.sort(function(a, b) {
    var aText = (a.children[colIdx] || {}).textContent || '';
    var bText = (b.children[colIdx] || {}).textContent || '';
    var aNum = parseFloat(aText);
    var bNum = parseFloat(bText);
    var cmp;
    if (!isNaN(aNum) && !isNaN(bNum)) {
      cmp = aNum - bNum;
    } else {
      cmp = aText.localeCompare(bText);
    }
    return isAsc ? -cmp : cmp;
  });

  rows.forEach(function(row) { tbody.appendChild(row); });
}

/* --- Collapsible diff hunks --- */
function toggleHunk(id) {
  var body = document.getElementById(id);
  if (body) {
    body.parentElement.classList.toggle('collapsed');
  }
}

/* --- Scroll-spy for TOC --- */
(function() {
  var links = document.querySelectorAll('.toc-link');
  if (links.length === 0) return;

  var targets = [];
  links.forEach(function(link) {
    var id = link.getAttribute('data-target');
    var el = document.getElementById(id);
    if (el) targets.push({ link: link, el: el });
  });

  function updateSpy() {
    var scrollY = window.scrollY + 80;
    var active = null;
    for (var i = targets.length - 1; i >= 0; i--) {
      if (targets[i].el.offsetTop <= scrollY) {
        active = targets[i];
        break;
      }
    }
    links.forEach(function(l) { l.classList.remove('active'); });
    if (active) active.link.classList.add('active');
  }

  window.addEventListener('scroll', updateSpy, { passive: true });
  updateSpy();
})();

/* --- Search --- */
(function() {
  var overlay = document.getElementById('search-overlay');
  var input = document.getElementById('search-input');
  var resultsDiv = document.getElementById('search-results');
  var countSpan = document.getElementById('search-count');
  var activeIdx = -1;
  var results = [];

  var searchItems = [];
  document.querySelectorAll('.block').forEach(function(block) {
    var header = block.querySelector('.block-header');
    var blockName = header ? header.textContent : '';
    block.querySelectorAll('p, h1, h2, h3, .list-item, .code-block code, td, th').forEach(function(el) {
      var text = el.textContent || '';
      if (text.trim()) {
        searchItems.push({ text: text.trim(), el: el, blockName: blockName });
      }
    });
  });

  function openSearch() {
    overlay.classList.remove('hidden');
    input.value = '';
    input.focus();
    resultsDiv.innerHTML = '';
    countSpan.textContent = '';
    activeIdx = -1;
    clearHighlights();
  }

  function closeSearch() {
    overlay.classList.add('hidden');
    clearHighlights();
  }

  function clearHighlights() {
    document.querySelectorAll('.search-highlight').forEach(function(el) {
      var parent = el.parentNode;
      parent.replaceChild(document.createTextNode(el.textContent), el);
      parent.normalize();
    });
  }

  function doSearch(query) {
    resultsDiv.innerHTML = '';
    results = [];
    activeIdx = -1;
    if (!query || query.length < 2) {
      countSpan.textContent = '';
      return;
    }
    var lower = query.toLowerCase();
    searchItems.forEach(function(item) {
      var idx = item.text.toLowerCase().indexOf(lower);
      if (idx !== -1) {
        results.push(item);
      }
    });

    countSpan.textContent = results.length + ' match' + (results.length !== 1 ? 'es' : '');

    results.slice(0, 50).forEach(function(item, i) {
      var div = document.createElement('div');
      div.className = 'search-result';
      var text = item.text;
      var idx = text.toLowerCase().indexOf(lower);
      var start = Math.max(0, idx - 30);
      var end = Math.min(text.length, idx + query.length + 30);
      var escH = function(s) { var d = document.createElement('div'); d.appendChild(document.createTextNode(s)); return d.innerHTML; };
      var snippet = (start > 0 ? '...' : '') + escH(text.substring(start, idx)) +
        '<mark>' + escH(text.substring(idx, idx + query.length)) + '</mark>' +
        escH(text.substring(idx + query.length, end)) + (end < text.length ? '...' : '');
      div.innerHTML = snippet + '<div class="sr-context">' + escH(item.blockName || '') + '</div>';
      div.addEventListener('click', function() {
        navigateTo(i);
      });
      resultsDiv.appendChild(div);
    });
  }

  function navigateTo(idx) {
    if (idx < 0 || idx >= results.length) return;
    activeIdx = idx;
    var item = results[idx];
    closeSearch();
    item.el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    item.el.classList.add('search-highlight');
    setTimeout(function() { item.el.classList.remove('search-highlight'); }, 3000);
    resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
      el.classList.toggle('active', i === idx);
    });
  }

  if (input) {
    input.addEventListener('input', function() {
      doSearch(input.value);
    });
    input.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') {
        closeSearch();
      } else if (e.key === 'Enter') {
        if (results.length > 0) {
          navigateTo(activeIdx < 0 ? 0 : activeIdx);
        }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (results.length > 0) {
          activeIdx = (activeIdx + 1) % Math.min(results.length, 50);
          resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
            el.classList.toggle('active', i === activeIdx);
            if (i === activeIdx) el.scrollIntoView({ block: 'nearest' });
          });
        }
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (results.length > 0) {
          activeIdx = activeIdx <= 0 ? Math.min(results.length, 50) - 1 : activeIdx - 1;
          resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
            el.classList.toggle('active', i === activeIdx);
            if (i === activeIdx) el.scrollIntoView({ block: 'nearest' });
          });
        }
      }
    });
  }

  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape' && !overlay.classList.contains('hidden')) {
      closeSearch();
      return;
    }
    if (overlay.classList.contains('hidden') && (e.key === '/' || (e.ctrlKey && e.key === 'k'))) {
      if (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA')) return;
      e.preventDefault();
      openSearch();
    }
  });
})();
` + csvFilterScript()
}

// csvFilterScript returns JavaScript for CSV column filtering
func csvFilterScript() string {
	return `
/* --- CSV column filter --- */
(function() {
  var filters = document.querySelectorAll('.col-filter');
  if (filters.length === 0) return;
  var table = document.getElementById('csv-table');
  if (!table) return;
  var tbody = table.querySelector('tbody');
  var countEl = document.getElementById('csv-row-count');
  var totalRows = tbody ? tbody.querySelectorAll('tr').length : 0;

  function applyFilters() {
    var rows = tbody.querySelectorAll('tr');
    var visible = 0;
    rows.forEach(function(row) {
      var show = true;
      filters.forEach(function(f) {
        var col = parseInt(f.getAttribute('data-col'));
        var val = f.value.toLowerCase();
        if (val && row.children[col]) {
          var cell = row.children[col].textContent.toLowerCase();
          if (cell.indexOf(val) === -1) show = false;
        }
      });
      row.style.display = show ? '' : 'none';
      if (show) visible++;
    });
    if (countEl) countEl.textContent = 'Showing ' + visible + ' of ' + totalRows + ' rows';
  }

  filters.forEach(function(f) {
    f.addEventListener('input', applyFilters);
  });

  // Prevent search overlay from opening when typing in filter
  filters.forEach(function(f) {
    f.addEventListener('keydown', function(e) { e.stopPropagation(); });
  });
})();
`
}

// enhancedScript returns all JavaScript for the enhanced features
func enhancedScript() string {
	return `
/* --- SSE live reload --- */
var es = new EventSource('/events');
es.onmessage = function(e) { if (e.data === 'reload') location.reload(); };
es.onerror = function() { setTimeout(function() { location.reload(); }, 2000); };

/* --- Syntax highlighting --- */
document.querySelectorAll('.code-block pre code').forEach(function(el) {
  hljs.highlightElement(el);
});

/* --- Copy button --- */
function copyCode(btn) {
  var code = btn.parentElement.querySelector('code');
  var text = code.textContent || code.innerText;
  navigator.clipboard.writeText(text).then(function() {
    btn.classList.add('copied');
    btn.textContent = 'Copied';
    setTimeout(function() {
      btn.classList.remove('copied');
      btn.textContent = 'Copy';
    }, 1500);
  });
}

/* --- Table sorting --- */
function sortTable(th, colIdx) {
  var table = th.closest('table');
  var tbody = table.querySelector('tbody');
  var rows = Array.from(tbody.querySelectorAll('tr'));
  var isAsc = th.classList.contains('asc');

  // Reset all headers in this table
  table.querySelectorAll('.sortable-th').forEach(function(h) { h.classList.remove('asc', 'desc'); });

  if (isAsc) {
    th.classList.add('desc');
  } else {
    th.classList.add('asc');
  }

  rows.sort(function(a, b) {
    var aText = (a.children[colIdx] || {}).textContent || '';
    var bText = (b.children[colIdx] || {}).textContent || '';
    var aNum = parseFloat(aText);
    var bNum = parseFloat(bText);
    var cmp;
    if (!isNaN(aNum) && !isNaN(bNum)) {
      cmp = aNum - bNum;
    } else {
      cmp = aText.localeCompare(bText);
    }
    return isAsc ? -cmp : cmp;
  });

  rows.forEach(function(row) { tbody.appendChild(row); });
}

/* --- Collapsible diff hunks --- */
function toggleHunk(id) {
  var body = document.getElementById(id);
  if (body) {
    body.parentElement.classList.toggle('collapsed');
  }
}

/* --- Scroll-spy for TOC --- */
(function() {
  var links = document.querySelectorAll('.toc-link');
  if (links.length === 0) return;

  var targets = [];
  links.forEach(function(link) {
    var id = link.getAttribute('data-target');
    var el = document.getElementById(id);
    if (el) targets.push({ link: link, el: el });
  });

  function updateSpy() {
    var scrollY = window.scrollY + 80;
    var active = null;
    for (var i = targets.length - 1; i >= 0; i--) {
      if (targets[i].el.offsetTop <= scrollY) {
        active = targets[i];
        break;
      }
    }
    links.forEach(function(l) { l.classList.remove('active'); });
    if (active) active.link.classList.add('active');
  }

  window.addEventListener('scroll', updateSpy, { passive: true });
  updateSpy();
})();

/* --- Search --- */
(function() {
  var overlay = document.getElementById('search-overlay');
  var input = document.getElementById('search-input');
  var resultsDiv = document.getElementById('search-results');
  var countSpan = document.getElementById('search-count');
  var activeIdx = -1;
  var results = [];

  // Build searchable index from all text content
  var searchItems = [];
  document.querySelectorAll('.block').forEach(function(block) {
    var header = block.querySelector('.block-header');
    var blockName = header ? header.textContent : '';
    block.querySelectorAll('p, h1, h2, h3, .list-item, .code-block code, td, th').forEach(function(el) {
      var text = el.textContent || '';
      if (text.trim()) {
        searchItems.push({ text: text.trim(), el: el, blockName: blockName });
      }
    });
  });

  function openSearch() {
    overlay.classList.remove('hidden');
    input.value = '';
    input.focus();
    resultsDiv.innerHTML = '';
    countSpan.textContent = '';
    activeIdx = -1;
    clearHighlights();
  }

  function closeSearch() {
    overlay.classList.add('hidden');
    clearHighlights();
  }

  function clearHighlights() {
    document.querySelectorAll('.search-highlight').forEach(function(el) {
      var parent = el.parentNode;
      parent.replaceChild(document.createTextNode(el.textContent), el);
      parent.normalize();
    });
  }

  function doSearch(query) {
    resultsDiv.innerHTML = '';
    results = [];
    activeIdx = -1;
    if (!query || query.length < 2) {
      countSpan.textContent = '';
      return;
    }
    var lower = query.toLowerCase();
    searchItems.forEach(function(item) {
      var idx = item.text.toLowerCase().indexOf(lower);
      if (idx !== -1) {
        results.push(item);
      }
    });

    countSpan.textContent = results.length + ' match' + (results.length !== 1 ? 'es' : '');

    results.slice(0, 50).forEach(function(item, i) {
      var div = document.createElement('div');
      div.className = 'search-result';
      var text = item.text;
      var idx = text.toLowerCase().indexOf(lower);
      var start = Math.max(0, idx - 30);
      var end = Math.min(text.length, idx + query.length + 30);
      var escH = function(s) { var d = document.createElement('div'); d.appendChild(document.createTextNode(s)); return d.innerHTML; };
      var snippet = (start > 0 ? '...' : '') + escH(text.substring(start, idx)) +
        '<mark>' + escH(text.substring(idx, idx + query.length)) + '</mark>' +
        escH(text.substring(idx + query.length, end)) + (end < text.length ? '...' : '');
      div.innerHTML = snippet + '<div class="sr-context">' + escH(item.blockName || '') + '</div>';
      div.addEventListener('click', function() {
        navigateTo(i);
      });
      resultsDiv.appendChild(div);
    });
  }

  function navigateTo(idx) {
    if (idx < 0 || idx >= results.length) return;
    activeIdx = idx;
    var item = results[idx];
    closeSearch();
    item.el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    item.el.classList.add('search-highlight');
    setTimeout(function() { item.el.classList.remove('search-highlight'); }, 3000);
    // Update active class
    resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
      el.classList.toggle('active', i === idx);
    });
  }

  if (input) {
    input.addEventListener('input', function() {
      doSearch(input.value);
    });
    input.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') {
        closeSearch();
      } else if (e.key === 'Enter') {
        if (results.length > 0) {
          navigateTo(activeIdx < 0 ? 0 : activeIdx);
        }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (results.length > 0) {
          activeIdx = (activeIdx + 1) % Math.min(results.length, 50);
          resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
            el.classList.toggle('active', i === activeIdx);
            if (i === activeIdx) el.scrollIntoView({ block: 'nearest' });
          });
        }
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (results.length > 0) {
          activeIdx = activeIdx <= 0 ? Math.min(results.length, 50) - 1 : activeIdx - 1;
          resultsDiv.querySelectorAll('.search-result').forEach(function(el, i) {
            el.classList.toggle('active', i === activeIdx);
            if (i === activeIdx) el.scrollIntoView({ block: 'nearest' });
          });
        }
      }
    });
  }

  // Global keyboard: / or Ctrl+K to open search, Escape to close
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape' && !overlay.classList.contains('hidden')) {
      closeSearch();
      return;
    }
    if (overlay.classList.contains('hidden') && (e.key === '/' || (e.ctrlKey && e.key === 'k'))) {
      if (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA')) return;
      e.preventDefault();
      openSearch();
    }
  });
})();
` + csvFilterScript()
}
