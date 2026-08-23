package engine

import (
	"os"
	"strings"
)

// ContractParser implements Parser for contract-type markdown files
// Detected via frontmatter type: "contract"
type ContractParser struct{}

// Detect checks if file has contract frontmatter type
func (p *ContractParser) Detect(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	fm, _ := ParseFrontmatter(string(content))
	return fm.Type == "contract"
}

// Parse reads contract content and returns a single block with ContractData
func (p *ContractParser) Parse(content string) []Block {
	fm, body := ParseFrontmatter(content)

	preamble, clauses := parseContractClauses(body)

	title := "Contract"
	if fm.Title != "" {
		title = fm.Title
	}

	parties := fm.Raw["parties"]
	effective := fm.Raw["effective"]

	block := Block{
		Name:        title,
		Content:     body,
		Pages:       []string{body},
		TotalPages:  1,
		ContentType: BlockContentContract,
		Data: &ContractData{
			Preamble:  preamble,
			Clauses:   clauses,
			Parties:   parties,
			Effective: effective,
		},
	}

	// Also build a TOC-friendly representation: store clause titles
	var tocParts []string
	for _, c := range clauses {
		tocParts = append(tocParts, c.ID+". "+c.Title)
	}
	if len(tocParts) > 0 {
		block.PageMeta = []string{strings.Join(tocParts, "\n")}
	}

	return []Block{block}
}
