package engine

import (
	"strings"
)

// Parser interface for extensibility - allows different file formats
type Parser interface {
	Parse(content string) []Block
	Detect(filePath string) bool // Auto-detect if this parser can handle the file
}

// FileParser extends Parser for binary/file-based content (images, video)
type FileParser interface {
	Parser
	ParseFile(filePath string, static bool) ([]Block, error)
}

// Block represents a markdown block with header and content
type Block struct {
	Name        string
	Content     string             // Full content (untruncated)
	LineNum     int
	Pages       []string           // Content split into pages
	TotalPages  int
	ContentType BlockContentType   // Default content type (for simple blocks)
	PageTypes   []BlockContentType // Per-page content type (for mixed content blocks)
	PageMeta      []string           // Per-page metadata (e.g., filename for diff pages)
	PageStartLine []int             // 1-indexed file line number for first content line of each page
	Data          interface{}        // Typed payload: *ImageData, *VideoData, *CsvData, etc.
}

// LinesPerPage is the fixed number of lines per page in e-reader mode
// TODO: Move to constants.go
const LinesPerPage = 50

// MarkdownParser implements Parser for markdown files
type MarkdownParser struct{}

// Detect checks if file is markdown
func (p *MarkdownParser) Detect(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".md") ||
		strings.HasSuffix(strings.ToLower(filePath), ".markdown")
}

// Parse reads a markdown file and extracts blocks
func (p *MarkdownParser) Parse(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	var currentBlockLines []string
	var currentHeader string
	var blockStartLine int

	for i, line := range lines {
		// Check if line is a top-level (#) or second-level (##) header
		isTopLevelHeader := strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ")
		isSecondLevelHeader := strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ")

		if isTopLevelHeader || isSecondLevelHeader {
			// Save previous block if exists
			if currentHeader != "" {
				block := createBlock(currentHeader, currentBlockLines, blockStartLine)
				blocks = append(blocks, block)
			}

			// Start new block
			if isTopLevelHeader {
				currentHeader = strings.TrimPrefix(line, "# ")
			} else {
				currentHeader = strings.TrimPrefix(line, "## ")
			}
			currentHeader = strings.TrimSpace(currentHeader)
			currentBlockLines = []string{}
			blockStartLine = i
		} else if currentHeader != "" {
			// Accumulate content for current block
			currentBlockLines = append(currentBlockLines, line)
		}
	}

	// Don't forget the last block
	if currentHeader != "" {
		block := createBlock(currentHeader, currentBlockLines, blockStartLine)
		blocks = append(blocks, block)
	}

	return blocks
}

// ParseContinuous treats markdown as continuous flow without header-based block cuts
// Pages are sized to fit the terminal: min(termHeight, maxLines)
// Tracks header breadcrumbs for each page (e.g., "Title > Section")
func (p *MarkdownParser) ParseContinuous(content string, termHeight int) []Block {
	maxLines := 50
	linesPerPage := termHeight - 4 // Reserve space for header/status
	if linesPerPage < 10 {
		linesPerPage = 10
	}
	if linesPerPage > maxLines {
		linesPerPage = maxLines
	}

	lines := strings.Split(content, "\n")

	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return []Block{{
			Name:        "Document",
			Content:     "",
			Pages:       []string{""},
			TotalPages:  1,
			ContentType: BlockContentPlain,
		}}
	}

	// Build header index: for each line, track active h1 and h2
	type headerState struct {
		h1 string
		h2 string
	}
	headerAtLine := make([]headerState, len(lines))
	var currentH1, currentH2 string

	for i, line := range lines {
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			currentH1 = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			currentH2 = "" // Reset h2 when new h1
		} else if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			currentH2 = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
		headerAtLine[i] = headerState{h1: currentH1, h2: currentH2}
	}

	// Split into pages, never cutting a fenced code block in half
	pages, pageStarts := splitIntoPagesFenceAware(lines, linesPerPage)

	// Build breadcrumb for each page based on first line of page
	pageMeta := make([]string, len(pages))
	for i := range pages {
		lineIndex := pageStarts[i]
		if lineIndex < len(headerAtLine) {
			state := headerAtLine[lineIndex]
			if state.h1 != "" && state.h2 != "" {
				pageMeta[i] = state.h1 + " > " + state.h2
			} else if state.h1 != "" {
				pageMeta[i] = state.h1
			} else if state.h2 != "" {
				pageMeta[i] = state.h2
			} else {
				pageMeta[i] = "Document"
			}
		}
	}

	// Compute PageStartLine for line number display
	pageStartLine := make([]int, len(pages))
	for i := range pageStartLine {
		pageStartLine[i] = 1 + pageStarts[i]
	}

	return []Block{{
		Name:          pageMeta[0], // First page breadcrumb as block name
		Content:       content,
		LineNum:       0,
		Pages:         pages,
		TotalPages:    len(pages),
		ContentType:   BlockContentPlain,
		PageMeta:      pageMeta, // Breadcrumb for each page
		PageStartLine: pageStartLine,
	}}
}

// createBlock constructs a Block from accumulated lines
func createBlock(header string, contentLines []string, lineNum int) Block {
	// Remove trailing empty lines
	for len(contentLines) > 0 && contentLines[len(contentLines)-1] == "" {
		contentLines = contentLines[:len(contentLines)-1]
	}

	// Join content lines
	fullContent := strings.Join(contentLines, "\n")

	// Check if content is a diff - if so, paginate by hunks
	var pages []string
	contentType := DetectBlockContentType(fullContent)
	if contentType == BlockContentDiff {
		pages = splitDiffIntoHunkPages(fullContent)
	} else {
		pages = splitIntoPages(contentLines, LinesPerPage)
	}

	// Compute PageStartLine for line number display (skip diff blocks)
	var pageStartLine []int
	if contentType != BlockContentDiff {
		pageStartLine = make([]int, len(pages))
		for i := range pageStartLine {
			pageStartLine[i] = lineNum + 2 + i*LinesPerPage
		}
	}

	return Block{
		Name:          header,
		Content:       fullContent,
		LineNum:       lineNum,
		Pages:         pages,
		TotalPages:    len(pages),
		ContentType:   contentType,
		PageStartLine: pageStartLine,
	}
}

// splitDiffIntoHunkPages splits diff content into pages where each page is one hunk
func splitDiffIntoHunkPages(content string) []string {
	hunks := ParseHunks(content)
	if len(hunks) == 0 {
		return []string{content}
	}

	// Each hunk becomes a "page" - but we store the full content
	// and use the page index to select which hunk to render
	pages := make([]string, len(hunks))
	for i := range hunks {
		// Store a placeholder - the actual hunk rendering happens in FormatDiffBlock
		pages[i] = content
	}
	return pages
}

// splitIntoPages splits content lines into pages of fixed size
func splitIntoPages(lines []string, linesPerPage int) []string {
	if len(lines) == 0 {
		return []string{""}
	}

	var pages []string
	for i := 0; i < len(lines); i += linesPerPage {
		end := i + linesPerPage
		if end > len(lines) {
			end = len(lines)
		}
		page := strings.Join(lines[i:end], "\n")
		pages = append(pages, page)
	}

	if len(pages) == 0 {
		pages = []string{""}
	}

	return pages
}

// splitIntoPagesFenceAware splits lines into pages of at most linesPerPage,
// never breaking inside a fenced code block. Returns the pages alongside the
// 0-indexed first line of each, since pages are no longer uniform.
func splitIntoPagesFenceAware(lines []string, linesPerPage int) ([]string, []int) {
	if len(lines) == 0 {
		return []string{""}, []int{0}
	}

	var pages []string
	var starts []int

	for i := 0; i < len(lines); {
		end := i + linesPerPage
		if end >= len(lines) {
			end = len(lines)
		} else {
			end = adjustPageBreak(lines, i, end)
		}
		pages = append(pages, strings.Join(lines[i:end], "\n"))
		starts = append(starts, i)
		i = end
	}

	return pages, starts
}

// adjustPageBreak moves a break that would land inside a code fence. A fence
// opened mid-page moves whole to the next page; one that opened at the top of
// the page (longer than a page) extends the page to its closing marker.
func adjustPageBreak(lines []string, start, end int) int {
	fenceStart := -1
	for i := start; i < end; i++ {
		if !isFenceMarker(lines[i]) {
			continue
		}
		if fenceStart < 0 {
			fenceStart = i
		} else {
			fenceStart = -1
		}
	}

	if fenceStart < 0 {
		return end
	}
	if fenceStart > start {
		return fenceStart
	}

	for i := end; i < len(lines); i++ {
		if isFenceMarker(lines[i]) {
			return i + 1
		}
	}
	return len(lines)
}

// isFenceMarker reports whether a line opens or closes a code fence
func isFenceMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// BlockIndex maps block names to blocks for quick lookup
type BlockIndex struct {
	Blocks    []Block
	NameIndex map[string]int
}

// NewBlockIndex creates an index from blocks
func NewBlockIndex(blocks []Block) *BlockIndex {
	index := &BlockIndex{
		Blocks:    blocks,
		NameIndex: make(map[string]int),
	}

	// Build name index (case-insensitive for easier lookup)
	for i, block := range blocks {
		lowerName := strings.ToLower(block.Name)
		index.NameIndex[lowerName] = i
	}

	return index
}

// FindBlock looks up a block by name (fuzzy match)
func (bi *BlockIndex) FindBlock(query string) *Block {
	query = strings.ToLower(strings.TrimSpace(query))

	// Exact match first
	if idx, ok := bi.NameIndex[query]; ok {
		return &bi.Blocks[idx]
	}

	// Fuzzy match: find blocks that contain the query
	var matches []int
	for i, block := range bi.Blocks {
		if strings.Contains(strings.ToLower(block.Name), query) {
			matches = append(matches, i)
		}
	}

	if len(matches) > 0 {
		// Return the first (best) match
		return &bi.Blocks[matches[0]]
	}

	return nil
}

// GetBlockByPosition returns block at given position in document
func (bi *BlockIndex) GetBlockByPosition(pos int) *Block {
	if pos >= 0 && pos < len(bi.Blocks) {
		return &bi.Blocks[pos]
	}
	return nil
}

// NextBlock returns the next block after the given block name
func (bi *BlockIndex) NextBlock(currentName string) *Block {
	currentName = strings.ToLower(currentName)
	if idx, ok := bi.NameIndex[currentName]; ok {
		if idx+1 < len(bi.Blocks) {
			return &bi.Blocks[idx+1]
		}
	}
	return nil
}

// PrevBlock returns the previous block before the given block name
func (bi *BlockIndex) PrevBlock(currentName string) *Block {
	currentName = strings.ToLower(currentName)
	if idx, ok := bi.NameIndex[currentName]; ok {
		if idx > 0 {
			return &bi.Blocks[idx-1]
		}
	}
	return nil
}

// GetAllBlockNames returns a list of all block names
func (bi *BlockIndex) GetAllBlockNames() []string {
	names := make([]string, len(bi.Blocks))
	for i, block := range bi.Blocks {
		names[i] = block.Name
	}
	return names
}
