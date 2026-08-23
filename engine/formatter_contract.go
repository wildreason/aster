package engine

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// contractClause represents a numbered section of a contract
type contractClause struct {
	ID    string // "1", "2", "3.1", etc.
	Level int    // 1 = ##, 2 = ###
	Title string // "Definitions", "Scope of Services"
	Body  string // raw markdown body text
}

// parseContractClauses splits markdown content into numbered clauses
func parseContractClauses(content string) (string, []contractClause) {
	var clauses []contractClause
	var current *contractClause
	var bodyLines []string
	var preambleLines []string
	inPreamble := true

	clauseHeadingRe := regexp.MustCompile(`^(#{2,3})\s+(\d+[\d.]*)\.\s+(.*)`)

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		m := clauseHeadingRe.FindStringSubmatch(line)
		if m != nil {
			// Save previous clause
			if current != nil {
				current.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
				clauses = append(clauses, *current)
				bodyLines = nil
			}
			inPreamble = false

			level := 1
			if m[1] == "###" {
				level = 2
			}

			current = &contractClause{
				ID:    m[2],
				Level: level,
				Title: m[3],
			}
		} else if inPreamble {
			preambleLines = append(preambleLines, line)
		} else if current != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	// Last clause
	if current != nil {
		current.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		clauses = append(clauses, *current)
	}

	return strings.TrimSpace(strings.Join(preambleLines, "\n")), clauses
}

// renderClauseBodyHTML converts clause body markdown to HTML paragraphs
func renderClauseBodyHTML(body string) string {
	var sb strings.Builder
	paragraphs := strings.Split(body, "\n\n")

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check for sub-headings (bold lines)
		if strings.HasPrefix(para, "**") && strings.HasSuffix(para, "**") {
			inner := para[2 : len(para)-2]
			sb.WriteString(fmt.Sprintf("<h4 class=\"clause-subheading\">%s</h4>\n", html.EscapeString(inner)))
			continue
		}

		// Handle "**Bold Title.** rest of text" pattern
		processed := processInlineHTML(para)
		sb.WriteString(fmt.Sprintf("<p>%s</p>\n", processed))
	}

	return sb.String()
}

// renderPreambleHTML converts preamble markdown to HTML
func renderPreambleHTML(preamble string) string {
	var sb strings.Builder
	lines := strings.Split(preamble, "\n")

	var currentBlock []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(currentBlock) > 0 {
				text := strings.Join(currentBlock, " ")
				if strings.HasPrefix(text, "# ") {
					title := strings.TrimPrefix(text, "# ")
					sb.WriteString(fmt.Sprintf("<h1 class=\"contract-title\">%s</h1>\n", html.EscapeString(title)))
				} else {
					sb.WriteString(fmt.Sprintf("<p>%s</p>\n", processInlineHTML(text)))
				}
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, trimmed)
		}
	}
	if len(currentBlock) > 0 {
		text := strings.Join(currentBlock, " ")
		if strings.HasPrefix(text, "# ") {
			title := strings.TrimPrefix(text, "# ")
			sb.WriteString(fmt.Sprintf("<h1 class=\"contract-title\">%s</h1>\n", html.EscapeString(title)))
		} else {
			sb.WriteString(fmt.Sprintf("<p>%s</p>\n", processInlineHTML(text)))
		}
	}

	return sb.String()
}
