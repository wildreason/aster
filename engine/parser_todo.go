package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TodoItem represents a single todo from the JSON file
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// TodoParser implements Parser for JSON todo files
type TodoParser struct{}

// Detect checks if content is a JSON todo array
func (p *TodoParser) Detect(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".json")
}

// Parse reads a JSON todo file and creates a single block
func (p *TodoParser) Parse(content string) []Block {
	var todos []TodoItem
	if err := json.Unmarshal([]byte(content), &todos); err != nil {
		return nil
	}

	if len(todos) == 0 {
		return nil
	}

	// Count completed
	completed := 0
	for _, t := range todos {
		if t.Status == "completed" {
			completed++
		}
	}

	// Build content
	var sb strings.Builder

	// Header with progress
	sb.WriteString(fmt.Sprintf("[yellow]todos[white] (%d/%d completed)\n\n", completed, len(todos)))

	// Render each todo
	// Use unicode symbols instead of brackets to avoid tview escaping issues
	for _, t := range todos {
		if t.Status == "completed" {
			sb.WriteString(fmt.Sprintf("[green]✓[-] %s\n", t.Content))
		} else if t.Status == "in_progress" {
			sb.WriteString(fmt.Sprintf("[cyan]→[-] %s\n", t.Content))
		} else {
			sb.WriteString(fmt.Sprintf("[#808080]○[-] %s\n", t.Content))
		}
	}

	blockContent := sb.String()

	return []Block{
		{
			Name:        fmt.Sprintf("[yellow]todos[-] [#808080]%d/%d[-]", completed, len(todos)),
			Content:     blockContent,
			LineNum:     0,
			Pages:       []string{blockContent},
			TotalPages:  1,
			ContentType: BlockContentPlain,
		},
	}
}
