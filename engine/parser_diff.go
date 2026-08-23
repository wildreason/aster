package engine

import (
	"strings"
)

// DiffParser implements Parser for diff/patch files
type DiffParser struct{}

// Detect checks if file is a diff/patch file
func (p *DiffParser) Detect(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".diff") ||
		strings.HasSuffix(lower, ".patch")
}

// Parse reads a diff file and creates blocks from hunks
func (p *DiffParser) Parse(content string) []Block {
	// First check if this is valid diff content
	if DetectBlockContentType(content) != BlockContentDiff {
		// Not a valid diff, return as single plain block
		return []Block{
			{
				Name:        "diff",
				Content:     content,
				LineNum:     0,
	
				Pages:       []string{content},
				TotalPages:  1,
				ContentType: BlockContentPlain,
			},
		}
	}

	// Parse into hunks
	hunks := ParseHunks(content)
	if len(hunks) == 0 {
		// Valid diff but no hunks parsed - show as single diff block
		return []Block{
			{
				Name:        "diff",
				Content:     content,
				LineNum:     0,
	
				Pages:       []string{content},
				TotalPages:  1,
				ContentType: BlockContentDiff,
				PageTypes:   []BlockContentType{BlockContentDiff},
				},
		}
	}

	// Get filename from diff
	filename := GetFileFromDiff(content)
	if filename == "" {
		filename = "diff"
	}

	// Create a single block with hunks as pages
	// Each page stores the full content; FormatDiffBlock handles hunk selection
	pages := make([]string, len(hunks))
	pageTypes := make([]BlockContentType, len(hunks))
	for i := range hunks {
		pages[i] = content
		pageTypes[i] = BlockContentDiff
	}

	return []Block{
		{
			Name:        filename,
			Content:     content,
			LineNum:     0,

			Pages:       pages,
			TotalPages:  len(hunks),
			ContentType: BlockContentDiff,
			PageTypes:   pageTypes,
		},
	}
}

// GetFileFromDiff extracts the filename from diff headers
func GetFileFromDiff(diffContent string) string {
	lines := strings.Split(diffContent, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimPrefix(line, "+++ ")
			path = strings.TrimPrefix(path, "b/")
			if idx := strings.Index(path, "\t"); idx != -1 {
				path = path[:idx]
			}
			return path
		}
	}

	return "file"
}
