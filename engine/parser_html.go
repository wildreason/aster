package engine

import (
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HTMLParser renders HTML documents by converting them to markdown and feeding
// the result through the existing markdown block pipeline.
//
// The tokenizer, tree builder and emitter below are hand-rolled against the
// standard library: aster's engine carries no HTML dependency and this parser
// does not add one. html.UnescapeString (stdlib) handles entities.
//
// Pipeline position: HTML -> markdown -> []Block -> Render. Converting to the
// markdown intermediate rather than to blocks directly means every existing
// output surface (terminal, --port, --html, --share) renders HTML for free.
type HTMLParser struct{}

// Detect checks if file is HTML
func (p *HTMLParser) Detect(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".html") ||
		strings.HasSuffix(lower, ".htm") ||
		strings.HasSuffix(lower, ".xhtml")
}

// Parse converts HTML to markdown, then splits it into header-delimited blocks
func (p *HTMLParser) Parse(content string) []Block {
	md, _ := HTMLToMarkdown(content)
	blocks := (&MarkdownParser{}).Parse(md)
	if len(blocks) == 0 {
		// No headers to cut on: fall back to continuous paging
		blocks = (&MarkdownParser{}).ParseContinuous(md, 24)
	}
	return blocks
}

// ParseContinuous converts HTML to markdown, then pages it as continuous flow
func (p *HTMLParser) ParseContinuous(content string, termHeight int) []Block {
	md, _ := HTMLToMarkdown(content)
	return (&MarkdownParser{}).ParseContinuous(md, termHeight)
}

// HTMLToMarkdown converts an HTML document to markdown. The second return is
// the document <title>, used as the page title on the web/static surfaces.
func HTMLToMarkdown(content string) (string, string) {
	nodes := buildHTMLTree(tokenizeHTML(content))
	title := htmlDocumentTitle(nodes)

	b := &mdBuilder{}
	renderHTMLChildren(nodes, b)
	lines := trimBlankEdges(b.lines)

	// Promote <title> to an h1 when the body never states one
	if title != "" && !hasTopLevelHeading(lines) {
		lines = append([]string{"# " + title, ""}, lines...)
	}
	return strings.Join(lines, "\n"), title
}

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

type htmlTokenKind int

const (
	htmlTokenText htmlTokenKind = iota
	htmlTokenStart
	htmlTokenEnd
)

type htmlAttr struct {
	Key string
	Val string
}

type htmlToken struct {
	Kind        htmlTokenKind
	Tag         string
	Attrs       []htmlAttr
	Text        string
	SelfClosing bool
}

// htmlVoidElements never have children and never carry a closing tag
var htmlVoidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// htmlRawTextElements hold text that is never parsed as markup
var htmlRawTextElements = map[string]bool{
	"script": true, "style": true, "textarea": true, "title": true,
}

// tokenizeHTML scans markup into a flat token stream. Malformed input never
// fails: anything unrecognized falls through as text.
func tokenizeHTML(s string) []htmlToken {
	var tokens []htmlToken
	i := 0

	for i < len(s) {
		lt := strings.IndexByte(s[i:], '<')
		if lt < 0 {
			tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: s[i:]})
			break
		}
		if lt > 0 {
			tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: s[i : i+lt]})
		}
		i += lt
		rest := s[i:]

		// Comments
		if strings.HasPrefix(rest, "<!--") {
			end := strings.Index(rest, "-->")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}

		// Doctype, CDATA, processing instructions
		if strings.HasPrefix(rest, "<!") || strings.HasPrefix(rest, "<?") {
			end := strings.IndexByte(rest, '>')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}

		// Closing tag
		if strings.HasPrefix(rest, "</") {
			name, after := scanHTMLTagName(rest[2:])
			if name == "" {
				tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: "<"})
				i++
				continue
			}
			_, consumed := scanHTMLTagBody(rest[2+after:])
			i += 2 + after + consumed
			tokens = append(tokens, htmlToken{Kind: htmlTokenEnd, Tag: name})
			continue
		}

		// Opening tag
		name, after := scanHTMLTagName(rest[1:])
		if name == "" {
			tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: "<"})
			i++
			continue
		}
		body, consumed := scanHTMLTagBody(rest[1+after:])
		attrs, selfClosing := parseHTMLAttrs(body)
		i += 1 + after + consumed
		tokens = append(tokens, htmlToken{
			Kind:        htmlTokenStart,
			Tag:         name,
			Attrs:       attrs,
			SelfClosing: selfClosing,
		})

		// Raw text elements swallow everything up to their close tag
		if htmlRawTextElements[name] && !selfClosing {
			idx := indexFoldASCII(s[i:], "</"+name)
			if idx < 0 {
				tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: s[i:]})
				i = len(s)
			} else {
				tokens = append(tokens, htmlToken{Kind: htmlTokenText, Text: s[i : i+idx]})
				i += idx
			}
		}
	}

	return tokens
}

// scanHTMLTagName reads a tag name, returning it lowercased plus the offset
// just past it. An empty name means this was not a tag at all.
func scanHTMLTagName(s string) (string, int) {
	j := 0
	for j < len(s) {
		c := s[j]
		if isASCIILetter(c) || (j > 0 && (isASCIIDigit(c) || c == '-' || c == '_' || c == ':')) {
			j++
			continue
		}
		break
	}
	if j == 0 {
		return "", 0
	}
	return strings.ToLower(s[:j]), j
}

// scanHTMLTagBody reads the attribute region up to the closing '>', respecting
// quoted values. Returns the region and the bytes consumed including the '>'.
func scanHTMLTagBody(s string) (string, int) {
	var quote byte
	for j := 0; j < len(s); j++ {
		c := s[j]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return s[:j], j + 1
		}
	}
	return s, len(s)
}

// parseHTMLAttrs handles key, key="value", key='value' and bare key=value
func parseHTMLAttrs(body string) ([]htmlAttr, bool) {
	var attrs []htmlAttr
	selfClosing := false
	i := 0

	for i < len(body) {
		for i < len(body) && isHTMLSpace(body[i]) {
			i++
		}
		if i >= len(body) {
			break
		}
		if body[i] == '/' {
			selfClosing = true
			i++
			continue
		}

		start := i
		for i < len(body) && !isHTMLSpace(body[i]) && body[i] != '=' && body[i] != '/' {
			i++
		}
		key := strings.ToLower(body[start:i])
		if key == "" {
			i++
			continue
		}

		j := i
		for j < len(body) && isHTMLSpace(body[j]) {
			j++
		}
		if j >= len(body) || body[j] != '=' {
			attrs = append(attrs, htmlAttr{Key: key})
			i = j
			continue
		}

		j++ // consume '='
		for j < len(body) && isHTMLSpace(body[j]) {
			j++
		}
		if j < len(body) && (body[j] == '"' || body[j] == '\'') {
			q := body[j]
			j++
			valStart := j
			for j < len(body) && body[j] != q {
				j++
			}
			attrs = append(attrs, htmlAttr{Key: key, Val: html.UnescapeString(body[valStart:j])})
			if j < len(body) {
				j++
			}
		} else {
			valStart := j
			for j < len(body) && !isHTMLSpace(body[j]) {
				j++
			}
			attrs = append(attrs, htmlAttr{Key: key, Val: html.UnescapeString(body[valStart:j])})
		}
		i = j
	}

	return attrs, selfClosing
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// indexFoldASCII is a case-insensitive strings.Index for ASCII needles
func indexFoldASCII(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	first := lowerASCII(substr[0])
	for i := 0; i+n <= len(s); i++ {
		if lowerASCII(s[i]) != first {
			continue
		}
		if strings.EqualFold(s[i:i+n], substr) {
			return i
		}
	}
	return -1
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// ---------------------------------------------------------------------------
// Tree
// ---------------------------------------------------------------------------

// htmlNode is an element when Tag is set, a text node otherwise
type htmlNode struct {
	Tag      string
	Attrs    []htmlAttr
	Text     string
	Children []*htmlNode
}

func (n *htmlNode) isText() bool { return n.Tag == "" }

func (n *htmlNode) attr(key string) string {
	for _, a := range n.Attrs {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// htmlClosesParagraph lists start tags that implicitly end an open <p>
var htmlClosesParagraph = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"details": true, "div": true, "dl": true, "fieldset": true,
	"figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hr": true, "li": true, "main": true, "nav": true,
	"ol": true, "p": true, "pre": true, "section": true, "table": true,
	"ul": true,
}

// htmlImpliesClose reports whether starting `start` implicitly closes `open`.
// This is what lets unclosed <li>, <td> and <p> tags parse the way a browser
// reads them rather than nesting forever.
func htmlImpliesClose(open, start string) bool {
	switch open {
	case "p":
		return htmlClosesParagraph[start]
	case "li":
		return start == "li"
	case "dt", "dd":
		return start == "dt" || start == "dd"
	case "td", "th":
		return start == "td" || start == "th" || start == "tr"
	case "tr":
		return start == "tr"
	case "thead", "tbody":
		return start == "tbody" || start == "tfoot"
	case "option":
		return start == "option" || start == "optgroup"
	}
	return false
}

// buildHTMLTree assembles tokens into a node tree, recovering from unclosed
// and stray tags instead of rejecting them
func buildHTMLTree(tokens []htmlToken) []*htmlNode {
	root := &htmlNode{Tag: "#root"}
	stack := []*htmlNode{root}

	for _, tok := range tokens {
		top := stack[len(stack)-1]

		switch tok.Kind {
		case htmlTokenText:
			if tok.Text != "" {
				top.Children = append(top.Children, &htmlNode{Text: tok.Text})
			}

		case htmlTokenStart:
			for len(stack) > 1 && htmlImpliesClose(stack[len(stack)-1].Tag, tok.Tag) {
				stack = stack[:len(stack)-1]
			}
			top = stack[len(stack)-1]
			node := &htmlNode{Tag: tok.Tag, Attrs: tok.Attrs}
			top.Children = append(top.Children, node)
			if !tok.SelfClosing && !htmlVoidElements[tok.Tag] {
				stack = append(stack, node)
			}

		case htmlTokenEnd:
			// Pop to the nearest matching open tag; ignore strays
			for j := len(stack) - 1; j > 0; j-- {
				if stack[j].Tag == tok.Tag {
					stack = stack[:j]
					break
				}
			}
		}
	}

	return root.Children
}

// htmlDocumentTitle finds the <title> text anywhere in the tree
func htmlDocumentTitle(nodes []*htmlNode) string {
	for _, n := range nodes {
		if n.isText() {
			continue
		}
		if n.Tag == "title" {
			return oneLine(html.UnescapeString(htmlRawText(n)))
		}
		if t := htmlDocumentTitle(n.Children); t != "" {
			return t
		}
	}
	return ""
}

// htmlRawText concatenates text descendants without collapsing whitespace
func htmlRawText(n *htmlNode) string {
	if n.isText() {
		return n.Text
	}
	if n.Tag == "br" {
		return "\n"
	}
	var sb strings.Builder
	for _, c := range n.Children {
		sb.WriteString(htmlRawText(c))
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Markdown emitter
// ---------------------------------------------------------------------------

// mdBuilder accumulates markdown lines under a prefix (list indent, quote
// marker) that nests as the walk descends
type mdBuilder struct {
	lines  []string
	prefix string
}

// blank appends a separator line, collapsing runs of them
func (b *mdBuilder) blank() {
	if len(b.lines) == 0 {
		return
	}
	if strings.TrimSpace(b.lines[len(b.lines)-1]) == "" {
		return
	}
	b.lines = append(b.lines, "")
}

func (b *mdBuilder) line(s string) {
	for _, part := range strings.Split(s, "\n") {
		b.lines = append(b.lines, strings.TrimRight(b.prefix+part, " \t"))
	}
}

// htmlInlineTags flow inside a paragraph. Unknown tags are treated as block
// containers so a nested document never collapses onto one line.
var htmlInlineTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "big": true,
	"br": true, "cite": true, "code": true, "data": true, "del": true,
	"dfn": true, "em": true, "font": true, "i": true, "img": true, "ins": true,
	"kbd": true, "label": true, "mark": true, "q": true, "s": true,
	"samp": true, "small": true, "span": true, "strike": true, "strong": true,
	"sub": true, "sup": true, "time": true, "tt": true, "u": true, "var": true,
	"wbr": true,
}

// htmlSkippedTags carry no reader-facing content
var htmlSkippedTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "canvas": true, "meta": true, "link": true, "base": true,
	"title": true, "iframe": true, "object": true, "embed": true,
	"select": true, "datalist": true, "input": true, "button": true,
	"textarea": true, "col": true, "colgroup": true, "param": true,
	"source": true, "track": true, "area": true, "map": true,
}

func isHTMLBlockNode(n *htmlNode) bool {
	return !n.isText() && !htmlInlineTags[n.Tag]
}

// renderHTMLChildren walks a child list, grouping consecutive inline content
// into paragraphs and dispatching block elements individually
func renderHTMLChildren(nodes []*htmlNode, b *mdBuilder) {
	var run []*htmlNode

	flush := func() {
		if len(run) == 0 {
			return
		}
		text := strings.TrimSpace(inlineHTML(run))
		run = nil
		if text == "" {
			return
		}
		b.blank()
		b.line(text)
		b.blank()
	}

	for _, n := range nodes {
		if isHTMLBlockNode(n) {
			flush()
			renderHTMLBlock(n, b)
			continue
		}
		run = append(run, n)
	}
	flush()
}

func renderHTMLBlock(n *htmlNode, b *mdBuilder) {
	if htmlSkippedTags[n.Tag] {
		return
	}

	switch n.Tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		text := oneLine(inlineHTML(n.Children))
		if text == "" {
			return
		}
		level := int(n.Tag[1] - '0')
		b.blank()
		b.line(strings.Repeat("#", level) + " " + text)
		b.blank()

	case "p", "figcaption", "address", "summary":
		text := strings.TrimSpace(inlineHTML(n.Children))
		if text == "" {
			return
		}
		b.blank()
		b.line(text)
		b.blank()

	case "hr":
		b.blank()
		b.line("---")
		b.blank()

	case "pre":
		renderHTMLPre(n, b)

	case "blockquote":
		sub := &mdBuilder{prefix: b.prefix + "> "}
		renderHTMLChildren(n.Children, sub)
		lines := trimBlankEdges(sub.lines)
		if len(lines) == 0 {
			return
		}
		b.blank()
		b.lines = append(b.lines, lines...)
		b.blank()

	case "ul", "ol":
		renderHTMLList(n, b, n.Tag == "ol")

	case "dl":
		renderHTMLDefList(n, b)

	case "table":
		renderHTMLTable(n, b)

	default:
		renderHTMLChildren(n.Children, b)
	}
}

// renderHTMLPre emits a fenced code block, preserving whitespace verbatim
func renderHTMLPre(n *htmlNode, b *mdBuilder) {
	target := n
	lang := htmlCodeLanguage(n)
	if code := firstChildElement(n, "code"); code != nil {
		target = code
		if l := htmlCodeLanguage(code); l != "" {
			lang = l
		}
	}

	text := html.UnescapeString(htmlRawText(target))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Trim(text, "\n")
	if strings.TrimSpace(text) == "" {
		return
	}

	b.blank()
	b.line("```" + lang)
	for _, l := range strings.Split(text, "\n") {
		b.line(l)
	}
	b.line("```")
	b.blank()
}

// htmlCodeLanguage reads a fence language from class="language-go" style hints
func htmlCodeLanguage(n *htmlNode) string {
	for _, class := range strings.Fields(n.attr("class")) {
		lower := strings.ToLower(class)
		for _, prefix := range []string{"language-", "lang-", "highlight-"} {
			if strings.HasPrefix(lower, prefix) {
				return strings.TrimPrefix(lower, prefix)
			}
		}
	}
	if l := n.attr("data-lang"); l != "" {
		return strings.ToLower(l)
	}
	return ""
}

func renderHTMLList(n *htmlNode, b *mdBuilder, ordered bool) {
	items := childElements(n, "li")
	if len(items) == 0 {
		return
	}

	index := 1
	if start := n.attr("start"); start != "" {
		if v, err := strconv.Atoi(start); err == nil {
			index = v
		}
	}

	b.blank()
	for _, li := range items {
		marker := "- "
		if ordered {
			marker = strconv.Itoa(index) + ". "
			index++
		}
		renderHTMLListItem(li, b, marker)
	}
	b.blank()
}

// renderHTMLListItem puts the item's leading content on the marker line and
// indents anything nested underneath it
func renderHTMLListItem(li *htmlNode, b *mdBuilder, marker string) {
	var head, tail []*htmlNode
	for _, c := range li.Children {
		// Everything up to the first nested block is the item's own text;
		// a leading <p> still counts as that text.
		if len(tail) == 0 && (!isHTMLBlockNode(c) || c.Tag == "p") {
			head = append(head, c)
			continue
		}
		tail = append(tail, c)
	}

	text := oneLine(inlineHTML(head))
	if text == "" && len(tail) == 0 {
		return
	}
	b.line(marker + text)

	if len(tail) > 0 {
		sub := &mdBuilder{prefix: b.prefix + strings.Repeat(" ", len(marker))}
		renderHTMLChildren(tail, sub)
		b.lines = append(b.lines, trimBlankEdges(sub.lines)...)
	}
}

func renderHTMLDefList(n *htmlNode, b *mdBuilder) {
	var emitted bool
	for _, c := range n.Children {
		if c.isText() {
			continue
		}
		text := oneLine(inlineHTML(c.Children))
		if text == "" {
			continue
		}
		switch c.Tag {
		case "dt":
			if !emitted {
				b.blank()
				emitted = true
			}
			b.line("**" + text + "**")
		case "dd":
			if !emitted {
				b.blank()
				emitted = true
			}
			b.line("  " + text)
		}
	}
	if emitted {
		b.blank()
	}
}

// renderHTMLTable emits a markdown table, which the block pipeline then
// detects as BlockContentTable and renders with borders
func renderHTMLTable(n *htmlNode, b *mdBuilder) {
	var header []string
	var rows [][]string
	var caption string

	var collect func(node *htmlNode)
	collect = func(node *htmlNode) {
		for _, c := range node.Children {
			if c.isText() {
				continue
			}
			switch c.Tag {
			case "thead", "tbody", "tfoot":
				collect(c)
			case "caption":
				if caption == "" {
					caption = oneLine(inlineHTML(c.Children))
				}
			case "tr":
				var cells []string
				allHeader := true
				for _, cell := range c.Children {
					if cell.Tag != "td" && cell.Tag != "th" {
						continue
					}
					if cell.Tag != "th" {
						allHeader = false
					}
					cells = append(cells, htmlTableCell(cell))
				}
				if len(cells) == 0 {
					continue
				}
				if allHeader && header == nil && len(rows) == 0 {
					header = cells
					continue
				}
				rows = append(rows, cells)
			}
		}
	}
	collect(n)

	if header == nil {
		if len(rows) == 0 {
			return
		}
		// Markdown tables require a header row; promote the first one
		header = rows[0]
		rows = rows[1:]
	}

	cols := len(header)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}

	b.blank()
	if caption != "" {
		b.line("**" + caption + "**")
		b.blank()
	}
	b.line(htmlTableRow(header, cols))
	b.line("|" + strings.Repeat(" --- |", cols))
	for _, r := range rows {
		b.line(htmlTableRow(r, cols))
	}
	b.blank()
}

func htmlTableRow(cells []string, cols int) string {
	padded := make([]string, cols)
	for i := 0; i < cols; i++ {
		if i < len(cells) {
			padded[i] = cells[i]
		}
	}
	return "| " + strings.Join(padded, " | ") + " |"
}

func htmlTableCell(cell *htmlNode) string {
	text := oneLine(inlineHTML(cell.Children))
	return strings.ReplaceAll(text, "|", "\\|")
}

// ---------------------------------------------------------------------------
// Inline rendering
// ---------------------------------------------------------------------------

func inlineHTML(nodes []*htmlNode) string {
	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(inlineHTMLNode(n))
	}
	return sb.String()
}

func inlineHTMLNode(n *htmlNode) string {
	if n.isText() {
		return collapseHTMLSpace(html.UnescapeString(n.Text))
	}
	if htmlSkippedTags[n.Tag] {
		return ""
	}

	switch n.Tag {
	case "br":
		return "\n"

	case "img":
		src := n.attr("src")
		alt := n.attr("alt")
		if src == "" {
			return alt
		}
		return "![" + alt + "](" + src + ")"

	case "a":
		text := strings.TrimSpace(inlineHTML(n.Children))
		href := strings.TrimSpace(n.attr("href"))
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return text
		}
		if text == "" {
			text = href
		}
		return "[" + text + "](" + href + ")"

	case "strong", "b":
		return wrapInline(inlineHTML(n.Children), "**")

	case "em", "i", "cite", "dfn", "var":
		return wrapInline(inlineHTML(n.Children), "*")

	case "del", "s", "strike":
		return wrapInline(inlineHTML(n.Children), "~~")

	case "code", "kbd", "samp", "tt":
		text := oneLine(inlineHTML(n.Children))
		if text == "" {
			return ""
		}
		if strings.Contains(text, "`") {
			return "`` " + text + " ``"
		}
		return "`" + text + "`"

	default:
		return inlineHTML(n.Children)
	}
}

// wrapInline applies a markdown marker to the text, keeping any surrounding
// whitespace outside the markers so emphasis still parses
func wrapInline(s, marker string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}
	lead := s[:len(s)-len(strings.TrimLeft(s, " \t\n"))]
	trail := s[len(strings.TrimRight(s, " \t\n")):]
	return lead + marker + trimmed + marker + trail
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// collapseHTMLSpace squeezes whitespace runs to a single space, preserving
// whether the text started or ended with one (word boundaries between tags)
func collapseHTMLSpace(s string) string {
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return " "
	}

	out := strings.Join(fields, " ")
	if first, _ := utf8.DecodeRuneInString(s); unicode.IsSpace(first) {
		out = " " + out
	}
	if last, _ := utf8.DecodeLastRuneInString(s); unicode.IsSpace(last) {
		out += " "
	}
	return out
}

// oneLine flattens text to a single trimmed line
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func firstChildElement(n *htmlNode, tag string) *htmlNode {
	for _, c := range n.Children {
		if c.Tag == tag {
			return c
		}
	}
	return nil
}

func childElements(n *htmlNode, tag string) []*htmlNode {
	var out []*htmlNode
	for _, c := range n.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

func trimBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func hasTopLevelHeading(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "# ") {
			return true
		}
	}
	return false
}

// isHTMLContent detects HTML from content alone, for stdin. Deliberately
// conservative: markdown with a stray <br> must not match.
func isHTMLContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !strings.HasPrefix(trimmed, "<") {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.HasPrefix(lower, "<body") {
		return true
	}

	// Otherwise require several distinct structural tags
	structural := []string{
		"<html", "<body", "<head", "<div", "<p>", "<p ", "<span", "<table",
		"<ul", "<ol", "<section", "<article", "<header", "<footer", "<main",
		"<h1", "<h2", "<h3", "<a ", "<img", "<pre",
	}
	distinct := 0
	for _, tag := range structural {
		if strings.Contains(lower, tag) {
			distinct++
		}
	}
	return distinct >= 2
}
