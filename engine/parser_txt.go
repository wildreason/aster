package engine

import (
	"regexp"
	"strings"
)

// extractCommandFromStyledLine extracts the command text from a styled line
// Handles formats like: [white:#303030] command [-:-:-]
func extractCommandFromStyledLine(line string) string {
	// Remove tview color tags: [anything] and [-:-:-]
	tviewTagRegex := regexp.MustCompile(`\[[^\]]*\]`)
	cleaned := tviewTagRegex.ReplaceAllString(line, "")
	return strings.TrimSpace(cleaned)
}

// TxtParser implements Parser for plain text / shell output files
type TxtParser struct{}

// Detect checks if file is txt
func (p *TxtParser) Detect(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".txt")
}

// Parse reads a txt file and extracts blocks
// Each "shell" line starts a new block, followed by command on next line
func (p *TxtParser) Parse(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	var currentBlock *Block
	var currentLines []string
	blockNum := 0
	i := 0

	for i < len(lines) {
		line := lines[i]
		// New format: "shell" line followed by command
		if strings.TrimSpace(line) == "shell" {
			// Save previous block
			if currentBlock != nil && len(currentLines) > 0 {
				currentBlock.Content = strings.Join(currentLines, "\n")
				currentBlock.Pages = splitIntoPages(currentLines, LinesPerPage)
				currentBlock.TotalPages = len(currentBlock.Pages)
				blocks = append(blocks, *currentBlock)
			}

			// Start new block
			blockNum++
			// Get command from next line (strip tview color tags)
			var command string
			if i+1 < len(lines) {
				command = extractCommandFromStyledLine(lines[i+1])
			}
			if command == "" {
				command = "shell"
			}
			// Truncate long command names
			if len(command) > 40 {
				command = command[:40] + "..."
			}

			currentBlock = &Block{
				Name:        command,
				LineNum:     blockNum,
				ContentType: BlockContentShell,
			}
			// Start content with command line (skip "shell" line)
			if i+1 < len(lines) {
				currentLines = []string{lines[i+1]}
				i++ // Skip command line since we've added it
			} else {
				currentLines = []string{}
			}
		} else if strings.HasPrefix(line, "$ ") {
			// Old format: "$ command (timestamp)" - backward compatibility
			// Save previous block
			if currentBlock != nil && len(currentLines) > 0 {
				currentBlock.Content = strings.Join(currentLines, "\n")
				currentBlock.Pages = splitIntoPages(currentLines, LinesPerPage)
				currentBlock.TotalPages = len(currentBlock.Pages)
				blocks = append(blocks, *currentBlock)
			}

			// Start new block
			blockNum++
			// Extract command name (before timestamp)
			name := line[2:] // Remove "$ "
			if idx := strings.Index(name, " ("); idx > 0 {
				name = name[:idx]
			}
			if len(name) > 40 {
				name = name[:40] + "..."
			}

			currentBlock = &Block{
				Name:        name,
				LineNum:     blockNum,
				ContentType: BlockContentShell,
			}
			currentLines = []string{line}
		} else if currentBlock != nil {
			currentLines = append(currentLines, line)
		} else {
			// Content before first shell block - create default block
			if strings.TrimSpace(line) != "" {
				blockNum++
				currentBlock = &Block{
					Name:        "Output",
					LineNum:     blockNum,
					ContentType: BlockContentShell,
				}
				currentLines = []string{line}
			}
		}
		i++
	}

	// Don't forget last block
	if currentBlock != nil && len(currentLines) > 0 {
		currentBlock.Content = strings.Join(currentLines, "\n")
		currentBlock.Pages = splitIntoPages(currentLines, LinesPerPage)
		currentBlock.TotalPages = len(currentBlock.Pages)
		blocks = append(blocks, *currentBlock)
	}

	// If no blocks found, create one with all content
	if len(blocks) == 0 && strings.TrimSpace(content) != "" {
		allLines := strings.Split(content, "\n")
		blocks = append(blocks, Block{
			Name:        "Output",
			Content:     content,
			LineNum:     1,
			Pages:       splitIntoPages(allLines, LinesPerPage),
			TotalPages:  len(splitIntoPages(allLines, LinesPerPage)),
			ContentType: BlockContentShell,
		})
	}

	return blocks
}
