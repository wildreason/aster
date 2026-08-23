package main

import (
	"strings"
	"testing"
)

func TestHTMLParserDetect(t *testing.T) {
	p := &HTMLParser{}
	for _, path := range []string{"page.html", "page.htm", "page.xhtml", "PAGE.HTML"} {
		if !p.Detect(path) {
			t.Errorf("expected %q to be detected as HTML", path)
		}
	}
	for _, path := range []string{"readme.md", "notes.txt", "data.json", "htmlish.md"} {
		if p.Detect(path) {
			t.Errorf("expected %q not to be detected as HTML", path)
		}
	}
}

func TestHTMLToMarkdownHeadings(t *testing.T) {
	md, _ := HTMLToMarkdown("<h1>One</h1><h2>Two</h2><h3>Three</h3><h6>Six</h6>")

	for _, want := range []string{"# One", "## Two", "### Three", "###### Six"} {
		if !strings.Contains(md, want) {
			t.Errorf("expected %q in output, got:\n%s", want, md)
		}
	}
}

func TestHTMLToMarkdownInlineFormatting(t *testing.T) {
	md, _ := HTMLToMarkdown(`<p>A <strong>bold</strong> and <em>italic</em> and ` +
		`<code>code()</code> and <del>gone</del> and <a href="https://x.dev">link</a> ` +
		`and <img src="a.png" alt="pic">.</p>`)

	want := "A **bold** and *italic* and `code()` and ~~gone~~ and " +
		"[link](https://x.dev) and ![pic](a.png)."
	if !strings.Contains(md, want) {
		t.Errorf("expected %q, got:\n%s", want, md)
	}
}

func TestHTMLToMarkdownKeepsWordBoundaries(t *testing.T) {
	// Whitespace between inline tags carries meaning and must survive collapse
	md, _ := HTMLToMarkdown("<p>one <b>two</b> three\n<i>four</i></p>")
	if !strings.Contains(md, "one **two** three *four*") {
		t.Errorf("word boundaries lost, got:\n%s", md)
	}
}

func TestHTMLToMarkdownGroupsInlineRuns(t *testing.T) {
	// Loose text and inline tags in a container form one paragraph, not several
	md, _ := HTMLToMarkdown("<div>Some <b>text</b> here</div>")
	if got := strings.Count(strings.TrimSpace(md), "\n"); got != 0 {
		t.Errorf("expected a single line, got %d newlines:\n%s", got, md)
	}
}

func TestHTMLToMarkdownEntities(t *testing.T) {
	md, _ := HTMLToMarkdown("<p>a &amp; b &lt;c&gt; &mdash; &#8212; &quot;q&quot;</p>")
	want := `a & b <c> — — "q"`
	if !strings.Contains(md, want) {
		t.Errorf("expected %q, got:\n%s", want, md)
	}
}

func TestHTMLToMarkdownLists(t *testing.T) {
	md, _ := HTMLToMarkdown(`
<ul>
  <li>first
  <li>second
    <ul><li>nested</li></ul>
  <li>third</li>
</ul>
<ol start="3"><li>three</li><li>four</li></ol>`)

	for _, want := range []string{"- first", "- second", "  - nested", "- third", "3. three", "4. four"} {
		if !strings.Contains(md, want) {
			t.Errorf("expected %q in output, got:\n%s", want, md)
		}
	}
}

func TestHTMLToMarkdownListItemWithParagraph(t *testing.T) {
	md, _ := HTMLToMarkdown("<ul><li><p>text</p><ul><li>sub</li></ul></li></ul>")
	if !strings.Contains(md, "- text") {
		t.Errorf("expected leading <p> to become the item text, got:\n%s", md)
	}
	if !strings.Contains(md, "  - sub") {
		t.Errorf("expected indented nested item, got:\n%s", md)
	}
}

func TestHTMLToMarkdownTable(t *testing.T) {
	md, _ := HTMLToMarkdown(`
<table>
  <caption>Cap</caption>
  <thead><tr><th>A</th><th>B</th></tr></thead>
  <tbody><tr><td>1</td><td>x | y</td></tr></tbody>
</table>`)

	for _, want := range []string{"**Cap**", "| A | B |", "| --- | --- |", `| 1 | x \| y |`} {
		if !strings.Contains(md, want) {
			t.Errorf("expected %q in output, got:\n%s", want, md)
		}
	}
}

func TestHTMLToMarkdownTablePromotesFirstRow(t *testing.T) {
	// Markdown tables need a header; a th-less table promotes its first row
	md, _ := HTMLToMarkdown("<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>")
	if !strings.Contains(md, "| a | b |") || !strings.Contains(md, "| --- | --- |") {
		t.Errorf("expected promoted header row, got:\n%s", md)
	}
}

func TestHTMLToMarkdownRaggedTable(t *testing.T) {
	md, _ := HTMLToMarkdown("<table><tr><th>a</th></tr><tr><td>b</td><td>c</td></tr></table>")
	if !strings.Contains(md, "| a |  |") {
		t.Errorf("expected header padded to widest row, got:\n%s", md)
	}
}

func TestHTMLToMarkdownPreservesCodeWhitespace(t *testing.T) {
	md, _ := HTMLToMarkdown("<pre><code class=\"language-go\">func f() {\n    x := 1\n}</code></pre>")

	if !strings.Contains(md, "```go") {
		t.Errorf("expected go fence, got:\n%s", md)
	}
	if !strings.Contains(md, "    x := 1") {
		t.Errorf("expected indentation preserved, got:\n%s", md)
	}
}

func TestHTMLToMarkdownSkipsScriptAndStyle(t *testing.T) {
	md, _ := HTMLToMarkdown(`
<style>body { color: red; }</style>
<script>var x = "<h1>fake heading</h1>";</script>
<p>real</p>`)

	if strings.Contains(md, "color") || strings.Contains(md, "fake heading") || strings.Contains(md, "var x") {
		t.Errorf("script/style content leaked into output:\n%s", md)
	}
	if !strings.Contains(md, "real") {
		t.Errorf("expected body text, got:\n%s", md)
	}
}

func TestHTMLToMarkdownTitle(t *testing.T) {
	md, title := HTMLToMarkdown("<html><head><title>Doc Title</title></head><body><p>body</p></body></html>")
	if title != "Doc Title" {
		t.Errorf("expected title 'Doc Title', got %q", title)
	}
	if !strings.HasPrefix(md, "# Doc Title") {
		t.Errorf("expected title promoted to h1, got:\n%s", md)
	}
}

func TestHTMLToMarkdownTitleNotDuplicated(t *testing.T) {
	md, _ := HTMLToMarkdown("<html><head><title>T</title></head><body><h1>Real</h1></body></html>")
	if strings.Contains(md, "# T\n") {
		t.Errorf("expected no synthetic h1 when the body has one, got:\n%s", md)
	}
}

func TestHTMLToMarkdownBlockquote(t *testing.T) {
	md, _ := HTMLToMarkdown("<blockquote><p>quoted</p><p>again</p></blockquote>")
	if !strings.Contains(md, "> quoted") || !strings.Contains(md, "> again") {
		t.Errorf("expected quoted lines, got:\n%s", md)
	}
}

func TestHTMLToMarkdownDefinitionList(t *testing.T) {
	md, _ := HTMLToMarkdown("<dl><dt>Term</dt><dd>Meaning</dd></dl>")
	if !strings.Contains(md, "**Term**") || !strings.Contains(md, "  Meaning") {
		t.Errorf("expected term and definition, got:\n%s", md)
	}
}

func TestHTMLToMarkdownMalformedInput(t *testing.T) {
	// Unclosed tags, stray closers, comments, doctype, bare and unquoted
	// attributes: none of this may panic or swallow the text
	cases := []string{
		"<p>unclosed paragraph",
		"<div><p>nested unclosed<div>more",
		"</span>stray closer<p>text</p>",
		"<!-- comment --><p>after comment</p>",
		"<!DOCTYPE html><p>after doctype</p>",
		"<p class=lead data-x id='q' hidden>attrs</p>",
		"<p>unterminated <!-- comment",
		"<p>a < b and 3<4</p>",
		"<",
		"<<>>",
		"",
	}
	for _, in := range cases {
		md, _ := HTMLToMarkdown(in)
		_ = md // must not panic
	}

	md, _ := HTMLToMarkdown("<p>unclosed paragraph")
	if !strings.Contains(md, "unclosed paragraph") {
		t.Errorf("expected text from unclosed tag, got:\n%s", md)
	}
	md, _ = HTMLToMarkdown("<p class=lead data-x id='q' hidden>attrs</p>")
	if !strings.Contains(md, "attrs") {
		t.Errorf("expected text past unquoted attributes, got:\n%s", md)
	}
}

func TestHTMLToMarkdownDropsJavascriptURLs(t *testing.T) {
	md, _ := HTMLToMarkdown(`<a href="javascript:alert(1)">click</a>`)
	if strings.Contains(strings.ToLower(md), "javascript:") {
		t.Errorf("javascript: URL survived conversion:\n%s", md)
	}
	if !strings.Contains(md, "click") {
		t.Errorf("expected link text retained, got:\n%s", md)
	}
}

func TestHTMLParserProducesBlocks(t *testing.T) {
	blocks := (&HTMLParser{}).Parse("<h1>Title</h1><p>body text</p>")
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	if !strings.Contains(strings.Join(blocks[0].Pages, "\n"), "body text") {
		t.Errorf("expected body text in first block, got %+v", blocks[0].Pages)
	}
}

func TestIsHTMLContent(t *testing.T) {
	yes := []string{
		"<!DOCTYPE html><html><body><p>hi</p></body></html>",
		"<html><body>x</body></html>",
		"<div><p>one</p><p>two</p></div>",
	}
	for _, in := range yes {
		if !isHTMLContent(in) {
			t.Errorf("expected HTML detection for %q", in)
		}
	}

	no := []string{
		"# Markdown\n\nwith a <br> tag",
		"plain text",
		"",
		`{"json": true}`,
		"- list\n- items",
	}
	for _, in := range no {
		if isHTMLContent(in) {
			t.Errorf("expected no HTML detection for %q", in)
		}
	}
}

func TestParseTableCellsEscapedPipe(t *testing.T) {
	cells := parseTableCells(`| a | b \| c | d |`)
	want := []string{"a", "b | c", "d"}
	if len(cells) != len(want) {
		t.Fatalf("expected %d cells, got %d: %q", len(want), len(cells), cells)
	}
	for i := range want {
		if cells[i] != want[i] {
			t.Errorf("cell %d: expected %q, got %q", i, want[i], cells[i])
		}
	}
}

func TestSplitIntoPagesFenceAware(t *testing.T) {
	// A fence that would straddle the break moves whole to the next page
	lines := []string{"a", "b", "```go", "code", "```", "after"}
	pages, starts := splitIntoPagesFenceAware(lines, 4)

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d: %q", len(pages), pages)
	}
	if strings.Contains(pages[0], "```") {
		t.Errorf("page 1 must not open a fence it cannot close: %q", pages[0])
	}
	if !strings.Contains(pages[1], "```go") || !strings.Contains(pages[1], "code") {
		t.Errorf("expected whole fence on page 2, got %q", pages[1])
	}
	if starts[0] != 0 || starts[1] != 2 {
		t.Errorf("expected starts [0 2], got %v", starts)
	}
}

func TestSplitIntoPagesFenceLongerThanPage(t *testing.T) {
	// A fence opening at the top of a page extends the page to its close
	lines := []string{"```", "1", "2", "3", "4", "```", "after"}
	pages, _ := splitIntoPagesFenceAware(lines, 3)

	if !strings.HasSuffix(strings.TrimSpace(pages[0]), "```") {
		t.Errorf("expected page 1 to run to the closing fence, got %q", pages[0])
	}
	if len(pages) != 2 || !strings.Contains(pages[1], "after") {
		t.Errorf("expected trailing content on its own page, got %q", pages)
	}
}

func TestSplitIntoPagesFenceAwareUnfenced(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	pages, starts := splitIntoPagesFenceAware(lines, 2)
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d: %q", len(pages), pages)
	}
	if starts[0] != 0 || starts[1] != 2 || starts[2] != 4 {
		t.Errorf("expected starts [0 2 4], got %v", starts)
	}
}
