package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// DiffColors defines the color scheme for diff rendering
// Terminal style: bright highlights on dark backgrounds
type DiffColors struct {
	// Text colors
	AddedText   string // White text on added lines
	RemovedText string // White text on removed lines
	ContextText string // Gray for context lines
	HeaderText  string // Gray for headers

	// Background colors (dark terminal style)
	AddedBg   string // Dark green #2d5a2d
	RemovedBg string // Dark magenta #5a2d5a

	Reset string
}

// DefaultDiffColors returns the magenta/green terminal color scheme
func DefaultDiffColors() DiffColors {
	return DiffColors{
		// White text on dark backgrounds
		AddedText:   "\033[38;2;255;255;255m", // White
		RemovedText: "\033[38;2;255;255;255m", // White
		ContextText: "\033[38;2;128;128;128m", // Gray
		HeaderText:  "\033[38;2;128;128;128m", // Gray

		// Dark background colors
		AddedBg:   "\033[48;2;45;90;45m",  // #2d5a2d - Dark green
		RemovedBg: "\033[48;2;90;45;90m",  // #5a2d5a - Dark magenta

		Reset: "\033[0m",
	}
}

// DiffHunk represents a single hunk from a unified diff
type DiffHunk struct {
	Header   string   // The @@ line (we hide this in display)
	Lines    []DiffLine
	StartOld int      // Starting line in old file
	StartNew int      // Starting line in new file
}

// DiffLine represents a single line in a hunk
type DiffLine struct {
	Type    DiffLineType
	Content string
}

// DiffLineType indicates whether a line was added, removed, or context
type DiffLineType int

const (
	DiffContext DiffLineType = iota
	DiffAdded
	DiffRemoved
)

// DiffFormatter renders diff content with the designed visual style
type DiffFormatter struct {
	Colors          DiffColors
	Width           int
	ShowFuncContext bool
	CurrentHunk     int
	TotalHunks      int
}

// NewDiffFormatter creates a formatter with default settings
func NewDiffFormatter(width int) *DiffFormatter {
	return &DiffFormatter{
		Colors:          DefaultDiffColors(),
		Width:           width,
		ShowFuncContext: true,
	}
}

// ParseHunks extracts hunks from unified diff content
func ParseHunks(content string) []DiffHunk {
	lines := strings.Split(content, "\n")
	var hunks []DiffHunk
	var currentHunk *DiffHunk

	for _, line := range lines {
		// Skip file headers
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		// New hunk starts with @@
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			currentHunk = &DiffHunk{
				Header: line,
				Lines:  []DiffLine{},
			}
			// Parse line numbers from @@ -start,count +start,count @@
			parseHunkHeader(line, currentHunk)
			continue
		}

		// Add lines to current hunk
		if currentHunk != nil {
			var lineType DiffLineType
			var lineContent string

			if strings.HasPrefix(line, "+") {
				lineType = DiffAdded
				lineContent = line[1:] // Strip the + prefix
			} else if strings.HasPrefix(line, "-") {
				lineType = DiffRemoved
				lineContent = line[1:] // Strip the - prefix
			} else if strings.HasPrefix(line, " ") {
				lineType = DiffContext
				lineContent = line[1:] // Strip the space prefix
			} else {
				// Empty line or other content
				lineType = DiffContext
				lineContent = line
			}

			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    lineType,
				Content: lineContent,
			})
		}
	}

	// Don't forget the last hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// parseHunkHeader extracts line numbers from @@ -old,count +new,count @@
func parseHunkHeader(header string, hunk *DiffHunk) {
	re := regexp.MustCompile(`@@ -(\d+),?\d* \+(\d+),?\d* @@`)
	matches := re.FindStringSubmatch(header)
	if len(matches) >= 3 {
		fmt.Sscanf(matches[1], "%d", &hunk.StartOld)
		fmt.Sscanf(matches[2], "%d", &hunk.StartNew)
	}
}

// FormatHunk renders a single hunk with the designed visual style
// Returns only the diff content - header/footer handled by FormatDiffPage
func (f *DiffFormatter) FormatHunk(hunk DiffHunk, hunkIndex int, totalHunks int, filename string) string {
	var sb strings.Builder
	c := f.Colors

	// Render each line with full width backgrounds
	contentWidth := f.Width - 4 // Account for indent
	if contentWidth < 40 {
		contentWidth = 40
	}

	for _, line := range hunk.Lines {
		formattedLine := f.formatLine(line, contentWidth)
		sb.WriteString(formattedLine)
		sb.WriteString("\n")
	}

	// Add context info at bottom if available
	funcContext := f.detectFunctions(hunk)
	if f.ShowFuncContext && funcContext != "" {
		sb.WriteString(fmt.Sprintf("\n  %s%s%s", c.HeaderText, funcContext, c.Reset))
	}

	return sb.String()
}

// formatLine renders a single diff line with colors and padding
func (f *DiffFormatter) formatLine(line DiffLine, width int) string {
	c := f.Colors
	content := line.Content

	// Pad to full width for solid background blocks (iteration 4)
	padding := width - len(content)
	if padding < 0 {
		padding = 0
	}
	paddedContent := content + strings.Repeat(" ", padding)

	switch line.Type {
	case DiffAdded:
		// High contrast: dark green text on light green background
		return fmt.Sprintf("    %s%s%s%s", c.AddedBg, c.AddedText, paddedContent, c.Reset)

	case DiffRemoved:
		// High contrast: dark red text on light red background
		return fmt.Sprintf("    %s%s%s%s", c.RemovedBg, c.RemovedText, paddedContent, c.Reset)

	case DiffContext:
		// Gray text, no background
		return fmt.Sprintf("    %s%s%s", c.ContextText, content, c.Reset)

	default:
		return "    " + content
	}
}

// detectFunctions finds function/class definitions in hunk (iteration 5)
func (f *DiffFormatter) detectFunctions(hunk DiffHunk) string {
	var functions []string
	seen := make(map[string]bool)

	// Patterns for different languages
	goFunc := regexp.MustCompile(`func\s+([a-zA-Z0-9_]+)\s*\(`)
	pyFunc := regexp.MustCompile(`(?:def|class)\s+([a-zA-Z0-9_]+)`)
	jsFunc := regexp.MustCompile(`(?:function\s+([a-zA-Z0-9_]+)|([a-zA-Z0-9_]+)\s*=\s*\(|class\s+([a-zA-Z0-9_]+))`)

	for _, line := range hunk.Lines {
		content := line.Content

		// Go functions
		if matches := goFunc.FindStringSubmatch(content); len(matches) > 1 {
			name := matches[1] + "()"
			if !seen[name] {
				functions = append(functions, name)
				seen[name] = true
			}
		}

		// Python functions/classes
		if matches := pyFunc.FindStringSubmatch(content); len(matches) > 1 {
			name := matches[1] + "()"
			if !seen[name] {
				functions = append(functions, name)
				seen[name] = true
			}
		}

		// JavaScript/TypeScript
		if matches := jsFunc.FindStringSubmatch(content); len(matches) > 0 {
			for i := 1; i < len(matches); i++ {
				if matches[i] != "" {
					name := matches[i] + "()"
					if !seen[name] {
						functions = append(functions, name)
						seen[name] = true
					}
					break
				}
			}
		}
	}

	if len(functions) > 0 {
		return "affects: " + strings.Join(functions, " ")
	}
	return ""
}

// Format renders the entire diff content
func (f *DiffFormatter) Format(content string, filename string) string {
	hunks := ParseHunks(content)
	if len(hunks) == 0 {
		return content // Not a valid diff, return as-is
	}

	f.TotalHunks = len(hunks)

	// For now, render all hunks sequentially
	// In the TUI, this would be paginated
	var sb strings.Builder
	for i, hunk := range hunks {
		sb.WriteString(f.FormatHunk(hunk, i, len(hunks), filename))
		if i < len(hunks)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
