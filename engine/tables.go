package engine

import "strings"

// isTableLine reports whether a line looks like a markdown table row.
// Shared by the HTML renderer and (via alias) the TUI markdown formatter.
func isTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// Must have pipes and at least 2 cells
	return strings.Contains(trimmed, "|") && strings.Count(trimmed, "|") >= 2
}

// isTableSeparator checks if a line is a table separator (|---|---|)
func isTableSeparator(line string) bool {
	for _, ch := range line {
		if ch != '-' && ch != ':' && ch != '|' && ch != ' ' {
			return false
		}
	}
	return strings.Contains(line, "-")
}

// parseTableCells extracts cells from a table row
func parseTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.Trim(trimmed, "|")

	var cells []string
	var cur strings.Builder
	for i := 0; i < len(trimmed); i++ {
		// A backslash-escaped pipe is cell content, not a separator
		if trimmed[i] == '\\' && i+1 < len(trimmed) && trimmed[i+1] == '|' {
			cur.WriteByte('|')
			i++
			continue
		}
		if trimmed[i] == '|' {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(trimmed[i])
	}
	cells = append(cells, strings.TrimSpace(cur.String()))

	return cells
}
