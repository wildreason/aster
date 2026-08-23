package engine

import (
	"regexp"
	"strings"
)

// BlockContentType identifies the type of content within a block
type BlockContentType int

const (
	BlockContentPlain BlockContentType = iota
	BlockContentDiff
	BlockContentTable
	BlockContentCode
	BlockContentTree
	BlockContentJSON
	BlockContentYAML
	BlockContentCSV
	BlockContentImage
	BlockContentVideo
	BlockContentContract
	BlockContentTranscript
	BlockContentShell
)

// String returns a human-readable name for the content type
func (ct BlockContentType) String() string {
	switch ct {
	case BlockContentDiff:
		return "diff"
	case BlockContentTable:
		return "table"
	case BlockContentCode:
		return "code"
	case BlockContentTree:
		return "tree"
	case BlockContentJSON:
		return "json"
	case BlockContentYAML:
		return "yaml"
	case BlockContentCSV:
		return "csv"
	case BlockContentImage:
		return "image"
	case BlockContentVideo:
		return "video"
	case BlockContentContract:
		return "contract"
	case BlockContentTranscript:
		return "transcript"
	case BlockContentShell:
		return "shell"
	default:
		return "plain"
	}
}

// DetectBlockContentType analyzes content and returns its type
func DetectBlockContentType(content string) BlockContentType {
	// Check for diff/patch format
	if isDiff(content) {
		return BlockContentDiff
	}

	// Check for table format (markdown or ASCII)
	if isTable(content) {
		return BlockContentTable
	}

	// Check for tree format (file listings)
	if isTree(content) {
		return BlockContentTree
	}

	// Check for JSON
	if isJSON(content) {
		return BlockContentJSON
	}

	// Check for YAML
	if isYAML(content) {
		return BlockContentYAML
	}

	return BlockContentPlain
}

// isDiff checks if content looks like a unified diff
func isDiff(content string) bool {
	lines := strings.Split(content, "\n")

	// Look for diff markers
	hasHunkHeader := false
	additionCount := 0
	deletionCount := 0
	hasFileHeader := false

	for _, line := range lines {
		// Hunk header: @@ -1,5 +1,6 @@
		if strings.HasPrefix(line, "@@") && strings.Contains(line, "@@") {
			hasHunkHeader = true
		}
		// File headers: --- a/file or +++ b/file
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			hasFileHeader = true
		}
		// Addition lines (but not +++ file header)
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additionCount++
		}
		// Deletion lines (but not --- file header)
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletionCount++
		}
	}

	// Strict check: must have hunk header AND file headers AND actual changes
	// This prevents false positives from markdown with bullet points or code
	return hasHunkHeader && hasFileHeader && (additionCount > 0 || deletionCount > 0)
}

// isTable checks if content looks like a table
func isTable(content string) bool {
	lines := strings.Split(content, "\n")

	// Look for markdown table patterns: | col | col |
	pipeCount := 0
	separatorLine := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			pipeCount++
		}
		// Markdown table separator: |---|---|
		if regexp.MustCompile(`^\|[\s\-:]+\|`).MatchString(line) {
			separatorLine = true
		}
	}

	// Need at least header, separator, and one data row
	return pipeCount >= 3 && separatorLine
}

// isTree checks if content looks like a file tree
func isTree(content string) bool {
	lines := strings.Split(content, "\n")

	// Look for tree-drawing characters
	treeChars := 0
	for _, line := range lines {
		if strings.Contains(line, "├") ||
			strings.Contains(line, "└") ||
			strings.Contains(line, "│") {
			treeChars++
		}
	}

	// Significant portion should have tree chars
	return len(lines) > 2 && treeChars > len(lines)/2
}

// isJSON checks if content looks like JSON
func isJSON(content string) bool {
	trimmed := strings.TrimSpace(content)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

// isYAML checks if content looks like YAML
func isYAML(content string) bool {
	lines := strings.Split(content, "\n")

	// Look for YAML patterns: key: value, - list items, indentation
	yamlPatterns := 0
	keyValuePattern := regexp.MustCompile(`^\s*[\w\-]+:\s*.+$`)
	listPattern := regexp.MustCompile(`^\s*-\s+.+$`)

	for _, line := range lines {
		if keyValuePattern.MatchString(line) || listPattern.MatchString(line) {
			yamlPatterns++
		}
	}

	// Significant portion should match YAML patterns
	return len(lines) > 2 && yamlPatterns > len(lines)/2
}
