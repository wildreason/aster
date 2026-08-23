package engine

import (
	"encoding/csv"
	"strings"
)

// CsvParser implements Parser for CSV and TSV files
type CsvParser struct {
	Delimiter rune // ',' for CSV, '\t' for TSV
}

// Detect checks if file is CSV or TSV
func (p *CsvParser) Detect(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".tsv")
}

// Parse reads CSV/TSV content and returns a single block with table content type
func (p *CsvParser) Parse(content string) []Block {
	delimiter := p.Delimiter
	if delimiter == 0 {
		delimiter = detectCSVDelimiter(content)
	}

	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil || len(records) < 1 {
		// Fallback: treat as plain text
		return []Block{{
			Name:        "CSV",
			Content:     content,
			Pages:       []string{content},
			TotalPages:  1,
			ContentType: BlockContentPlain,
		}}
	}

	// Convert CSV records to markdown table format for reuse of existing table rendering
	mdTable := csvToMarkdownTable(records)

	return []Block{{
		Name:        "CSV",
		Content:     mdTable,
		Pages:       []string{mdTable},
		TotalPages:  1,
		ContentType: BlockContentCSV,
		Data:        &CsvData{Records: records},
	}}
}

// detectCSVDelimiter guesses whether content is comma or tab delimited
func detectCSVDelimiter(content string) rune {
	lines := strings.SplitN(content, "\n", 5)
	if len(lines) == 0 {
		return ','
	}

	tabs := 0
	commas := 0
	for _, line := range lines {
		tabs += strings.Count(line, "\t")
		commas += strings.Count(line, ",")
	}

	if tabs > commas {
		return '\t'
	}
	return ','
}

// csvToMarkdownTable converts CSV records to a markdown table string
func csvToMarkdownTable(records [][]string) string {
	if len(records) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header row
	sb.WriteString("| ")
	sb.WriteString(strings.Join(records[0], " | "))
	sb.WriteString(" |\n")

	// Separator
	sb.WriteString("|")
	for range records[0] {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Data rows
	for i := 1; i < len(records); i++ {
		sb.WriteString("| ")
		// Pad row to match header column count
		row := records[i]
		cells := make([]string, len(records[0]))
		for j := range cells {
			if j < len(row) {
				cells[j] = row[j]
			}
		}
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |\n")
	}

	return sb.String()
}

// isCSV checks if content looks like CSV (consistent comma-separated columns)
func isCSV(content string) bool {
	lines := strings.SplitN(content, "\n", 10)
	if len(lines) < 2 {
		return false
	}

	// Count fields in first few non-empty lines
	delimiter := detectCSVDelimiter(content)
	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true

	firstFields := -1
	consistent := 0
	for i := 0; i < 5; i++ {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if firstFields == -1 {
			firstFields = len(record)
			if firstFields < 2 {
				return false // Need at least 2 columns
			}
		}
		if len(record) == firstFields {
			consistent++
		}
	}

	return consistent >= 2
}
