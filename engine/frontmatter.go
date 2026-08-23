package engine

import "strings"

// Frontmatter holds parsed YAML frontmatter from a document
type Frontmatter struct {
	Title   string
	Created string
	Tags    []string
	Type    string // e.g. "contract" for Oberon-style interactive rendering
	Raw     map[string]string
}

// ParseFrontmatter extracts YAML frontmatter delimited by --- from content.
// Returns the parsed frontmatter and the remaining body.
// If no frontmatter is found, returns empty Frontmatter and original content.
func ParseFrontmatter(content string) (Frontmatter, string) {
	if !strings.HasPrefix(content, "---") {
		return Frontmatter{}, content
	}

	// Find closing delimiter
	rest := content[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	} else {
		return Frontmatter{}, content
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		return Frontmatter{}, content
	}

	fmBlock := rest[:closeIdx]
	body := rest[closeIdx+4:] // skip \n---
	// Strip leading newlines from body
	body = strings.TrimLeft(body, "\r\n")

	fm := Frontmatter{
		Raw: make(map[string]string),
	}

	for _, line := range strings.Split(fmBlock, "\n") {
		line = strings.TrimRight(line, "\r")
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		if key == "" {
			continue
		}

		fm.Raw[key] = value

		switch key {
		case "title":
			fm.Title = value
		case "created":
			fm.Created = value
		case "tags":
			fm.Tags = parseBracketList(value)
		case "type":
			fm.Type = value
		}
	}

	return fm, body
}

// parseBracketList parses "[a, b, c]" into []string{"a", "b", "c"}
func parseBracketList(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
