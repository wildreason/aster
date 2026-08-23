package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ContentType represents a type of content in JSONL
type ContentType struct {
	Name    string
	Count   int
	Enabled bool
}

// JSONLParser implements Parser for JSONL transcript files
type JSONLParser struct {
	Filters map[string]bool // Which content types to include

	// Decorate, when set, styles a transcript part for the surface that will
	// read Block.Content -- the CLI wires tview markup here for the TUI.
	// nil means plain text. HTML surfaces never read Content for transcripts
	// (they consume Block.Data *TranscriptData), so hosts leave this nil.
	Decorate func(partType, content, meta string) string
}

// plainPart is the nil-Decorate default: content with minimal, markup-free
// framing. Chosen to be readable if a surface ever does print it, and to
// carry the same information the TUI decoration carries.
func plainPart(partType, content, meta string) string {
	switch partType {
	case "diff":
		filename := meta
		if idx := strings.LastIndex(meta, "/"); idx >= 0 {
			filename = meta[idx+1:]
		}
		return "--- " + filename + " ---\n" + content
	case "question":
		return "[?] " + content
	default:
		return content
	}
}

// Detect checks if file is JSONL
func (p *JSONLParser) Detect(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".jsonl")
}

// ScanContentTypes scans JSONL content and returns available types with counts
func ScanContentTypes(content string) []ContentType {
	counts := make(map[string]int)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		// Categorize message types
		switch msgType {
		case "user":
			// Check if it's actual user text or tool result
			if message, ok := msg["message"].(map[string]interface{}); ok {
				if content, ok := message["content"]; ok {
					if _, isString := content.(string); isString {
						counts["user"]++
					} else if arr, isArr := content.([]interface{}); isArr {
						// Check first item type
						for _, item := range arr {
							if itemMap, ok := item.(map[string]interface{}); ok {
								if itemType, _ := itemMap["type"].(string); itemType == "tool_result" {
									counts["tool_result"]++
									// Also check for diff content
									if hasStructuredPatch(msg) {
										counts["diff"]++
									}
									break
								} else {
									counts["user"]++
									break
								}
							}
						}
					}
				}
			}
		case "assistant":
			counts["assistant"]++
		case "system":
			counts["system"]++
		default:
			// Group other types (file-history-snapshot, summary, etc.)
			counts["other"]++
		}
	}

	// Build result with sensible ordering and defaults
	var types []ContentType
	order := []string{"user", "assistant", "diff", "tool_result", "system", "other"}
	defaults := map[string]bool{"user": true, "assistant": true, "diff": true}

	for _, name := range order {
		if count, exists := counts[name]; exists && count > 0 {
			types = append(types, ContentType{
				Name:    name,
				Count:   count,
				Enabled: defaults[name],
			})
		}
	}

	return types
}

// hasStructuredPatch checks if a message has a non-empty structuredPatch
func hasStructuredPatch(msg map[string]interface{}) bool {
	toolUseResult, ok := msg["toolUseResult"].(map[string]interface{})
	if !ok {
		return false
	}
	patch, ok := toolUseResult["structuredPatch"].([]interface{})
	if !ok {
		return false
	}
	return len(patch) > 0
}

// extractStructuredPatch extracts and converts structuredPatch to unified diff format
func extractStructuredPatch(msg map[string]interface{}) string {
	toolUseResult, ok := msg["toolUseResult"].(map[string]interface{})
	if !ok {
		return ""
	}

	// Get file path for diff header
	filePath, _ := toolUseResult["filePath"].(string)
	if filePath == "" {
		filePath = "file"
	}

	patch, ok := toolUseResult["structuredPatch"].([]interface{})
	if !ok || len(patch) == 0 {
		return ""
	}

	var sb strings.Builder

	// Write diff header
	sb.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	// Process each hunk in the patch
	for _, hunkData := range patch {
		hunk, ok := hunkData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract hunk header info
		oldStart, _ := hunk["oldStart"].(float64)
		oldLines, _ := hunk["oldLines"].(float64)
		newStart, _ := hunk["newStart"].(float64)
		newLines, _ := hunk["newLines"].(float64)

		// Write hunk header
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			int(oldStart), int(oldLines), int(newStart), int(newLines)))

		// Write lines
		lines, ok := hunk["lines"].([]interface{})
		if !ok {
			continue
		}

		for _, lineData := range lines {
			line, ok := lineData.(string)
			if !ok {
				continue
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}


// Parse reads a JSONL file and extracts conversation blocks
func (p *JSONLParser) Parse(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	var currentTurn *ConversationTurn
	turnNumber := 0

	// Default filters if not set
	if p.Filters == nil {
		p.Filters = map[string]bool{"user": true, "assistant": true, "diff": true}
	}

	showUser := p.Filters["user"]
	showAssistant := p.Filters["assistant"]
	showDiff := p.Filters["diff"]
	showToolResult := p.Filters["tool_result"]

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse JSON object
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Skip invalid JSON lines
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		// Process based on filters
		if msgType == "user" {
			// Determine if this is user text or tool result
			isToolResult := p.isToolResultMessage(msg)

			// TOOL RESULTS: Add diffs and/or tool output to current turn
			if isToolResult {
				if currentTurn != nil {
					// Check for diff content (diffs come from tool results)
					if showDiff && hasStructuredPatch(msg) {
						diffContent := extractStructuredPatch(msg)
						if diffContent != "" {
							toolUseResult, _ := msg["toolUseResult"].(map[string]interface{})
							filePath, _ := toolUseResult["filePath"].(string)
							currentTurn.Parts = append(currentTurn.Parts, TurnPart{
								Type:    "diff",
								Content: diffContent,
								Meta:    filePath,
							})
						}
					}

					// Show tool result output if filter enabled
					if showToolResult {
						toolContent := p.ExtractToolResultContent(msg)
						if toolContent != "" {
							currentTurn.Parts = append(currentTurn.Parts, TurnPart{
								Type:    "tool_result",
								Content: toolContent,
							})
						}
					}
				}
				// Tool results don't create new turns
				continue
			}

			// REAL USER MESSAGE: Start a new turn
			if !showUser {
				continue
			}

			// Save previous turn if exists
			if currentTurn != nil {
				block := p.CreateTurnBlock(currentTurn, turnNumber)
				blocks = append(blocks, block)
			}

			// Start new turn with user message as first part
			turnNumber++
			userContent := p.ExtractUserContent(msg)
			if userContent != "" {
				currentTurn = &ConversationTurn{
					Parts:   []TurnPart{{Type: "user", Content: userContent}},
					LineNum: lineNum,
				}
			}
		} else if msgType == "assistant" && showAssistant && currentTurn != nil {
			// Add assistant response as a part of the current turn
			assistantContent := p.ExtractAssistantContent(msg)
			if assistantContent != "" {
				currentTurn.Parts = append(currentTurn.Parts, TurnPart{
					Type:    "assistant",
					Content: assistantContent,
				})
			}
		}
	}

	// Don't forget the last turn
	if currentTurn != nil {
		block := p.CreateTurnBlock(currentTurn, turnNumber)
		blocks = append(blocks, block)
	}

	return blocks
}

// GetMessageType returns the message type from a JSONL line ("user", "assistant", or "")
func (p *JSONLParser) GetMessageType(line string) string {
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ""
	}
	msgType, _ := msg["type"].(string)
	return msgType
}

// ParseLineInfo parses a JSONL line and returns the parsed message, type, and whether it's a tool result
func (p *JSONLParser) ParseLineInfo(line string) (map[string]interface{}, string, bool) {
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, "", false
	}
	msgType, _ := msg["type"].(string)
	isToolResult := msgType == "user" && p.isToolResultMessage(msg)
	return msg, msgType, isToolResult
}

// QuestionOption represents a single option in a question
type QuestionOption struct {
	Label       string
	Description string
}

// QuestionData represents an AskUserQuestion from Claude Code
type QuestionData struct {
	Question    string
	Header      string
	Options     []QuestionOption
	MultiSelect bool
}

// ExtractAskUserQuestion checks if an assistant message contains AskUserQuestion tool_use
// and extracts the first question (for backward compatibility)
func (p *JSONLParser) ExtractAskUserQuestion(msg map[string]interface{}) *QuestionData {
	questions := p.ExtractAllQuestions(msg)
	if len(questions) > 0 {
		return questions[0]
	}
	return nil
}

// ExtractAllQuestions extracts ALL questions from an AskUserQuestion tool_use
func (p *JSONLParser) ExtractAllQuestions(msg map[string]interface{}) []*QuestionData {
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	content := message["content"]
	if content == nil {
		return nil
	}

	contentArr, ok := content.([]interface{})
	if !ok {
		return nil
	}

	for _, item := range contentArr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)
		if itemType != "tool_use" {
			continue
		}

		name, _ := itemMap["name"].(string)
		if name != "AskUserQuestion" {
			continue
		}

		// Found AskUserQuestion - extract input
		input, ok := itemMap["input"].(map[string]interface{})
		if !ok {
			continue
		}

		questionsArr, ok := input["questions"].([]interface{})
		if !ok || len(questionsArr) == 0 {
			continue
		}

		// Extract ALL questions
		var result []*QuestionData
		for _, qItem := range questionsArr {
			q, ok := qItem.(map[string]interface{})
			if !ok {
				continue
			}

			data := &QuestionData{}
			data.Question, _ = q["question"].(string)
			data.Header, _ = q["header"].(string)
			data.MultiSelect, _ = q["multiSelect"].(bool)

			// Extract options
			if opts, ok := q["options"].([]interface{}); ok {
				for _, opt := range opts {
					if optMap, ok := opt.(map[string]interface{}); ok {
						label, _ := optMap["label"].(string)
						desc, _ := optMap["description"].(string)
						data.Options = append(data.Options, QuestionOption{
							Label:       label,
							Description: desc,
						})
					}
				}
			}
			result = append(result, data)
		}

		return result
	}

	return nil
}

// FormatQuestionContent formats QuestionData as display string
// Use index=0 and total=0 for single question (no Q1/N prefix)
func FormatQuestionContent(data *QuestionData) string {
	return FormatQuestionContentIndexed(data, 0, 0)
}

// FormatQuestionContentIndexed formats QuestionData with Q index/total prefix
func FormatQuestionContentIndexed(data *QuestionData, index int, total int) string {
	if data == nil {
		return ""
	}

	var content strings.Builder

	// Q index/total prefix for multi-question
	if total > 1 {
		content.WriteString(fmt.Sprintf("[yellow]Q%d/%d[-] ", index, total))
	}

	// Header
	if data.Header != "" {
		content.WriteString(fmt.Sprintf("[#808080]%s[-]\n", data.Header))
	}

	// Question text
	content.WriteString("\n")
	content.WriteString(data.Question)
	content.WriteString("\n\n")

	// Options
	for i, opt := range data.Options {
		content.WriteString(fmt.Sprintf("  [cyan]%d.[-] %s", i+1, opt.Label))
		if opt.Description != "" {
			content.WriteString(fmt.Sprintf(" - [#808080]%s[-]", opt.Description))
		}
		content.WriteString("\n")
	}

	// "Other" option hint
	content.WriteString(fmt.Sprintf("  [cyan]%d.[-] Other (custom text)\n", len(data.Options)+1))

	// Multi-select hint
	if data.MultiSelect {
		content.WriteString("\n[#808080](multi-select: e.g. 1,3)[-]\n")
	}

	return content.String()
}

// ExtractAssistantText extracts text content from an assistant message line
func (p *JSONLParser) ExtractAssistantText(line string) string {
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ""
	}
	return p.ExtractAssistantContent(msg)
}

// ParseSingleLine parses a single JSONL line and returns a block if it matches filters
// Used for follow mode where we process lines incrementally
func (p *JSONLParser) ParseSingleLine(line string, turnNumber int) *Block {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Default filters if not set
	if p.Filters == nil {
		p.Filters = map[string]bool{"user": true, "assistant": true, "diff": true}
	}

	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return nil
	}

	var blockName string
	var contentLines []string

	switch msgType {
	case "user":
		// Check if this is a tool result
		isToolResult := p.isToolResultMessage(msg)

		// TOOL RESULTS: Only show diffs, skip everything else
		if isToolResult {
			if p.Filters["diff"] && hasStructuredPatch(msg) {
				return p.createDiffBlock(msg, turnNumber, 0)
			}
			// Skip all other tool results - they don't create blocks
			return nil
		}

		// REAL USER MESSAGE
		if !p.Filters["user"] {
			return nil
		}
		userContent := p.ExtractUserContent(msg)
		if userContent == "" {
			return nil
		}
		blockName = fmt.Sprintf("block-%d", turnNumber)
		contentLines = append(contentLines, fmt.Sprintf("[cyan]U:[-] %s", userContent))

	case "assistant":
		if !p.Filters["assistant"] {
			return nil
		}
		assistantContent := p.ExtractAssistantContent(msg)
		if assistantContent == "" {
			return nil
		}
		blockName = fmt.Sprintf("block-%d", turnNumber)
		// Split assistant content into lines
		assistantLines := strings.Split(assistantContent, "\n")
		if len(assistantLines) > 0 {
			// First line with A: prefix
			contentLines = append(contentLines, "A: "+assistantLines[0])
			// Remaining lines without prefix
			if len(assistantLines) > 1 {
				contentLines = append(contentLines, assistantLines[1:]...)
			}
		}

	default:
		return nil
	}

	fullContent := strings.Join(contentLines, "\n")
	pages := splitIntoPages(contentLines, LinesPerPage)

	return &Block{
		Name:        blockName,
		Content:     fullContent,
		LineNum:     0,

		Pages:       pages,
		TotalPages:  len(pages),
		ContentType: BlockContentPlain, // User/assistant messages are plain text
	}
}

// isToolResultMessage checks if a user message is actually a tool result
func (p *JSONLParser) isToolResultMessage(msg map[string]interface{}) bool {
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return false
	}
	content := message["content"]
	if arr, isArr := content.([]interface{}); isArr {
		for _, item := range arr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, _ := itemMap["type"].(string); itemType == "tool_result" {
					return true
				}
			}
		}
	}
	return false
}

// ExtractToolResultContent extracts the output from a tool result message
// Uses ShellFormatter for proper formatting with ANSI stripping and truncation
func (p *JSONLParser) ExtractToolResultContent(msg map[string]interface{}) string {
	// Check toolUseResult and use ShellFormatter
	if toolUseResult, ok := msg["toolUseResult"].(map[string]interface{}); ok {
		output := ParseToolResult(toolUseResult)
		if output != nil {
			formatter := NewShellFormatter(0)
			return formatter.Format(output)
		}
	}

	// Fallback: extract from message.content tool_result
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return ""
	}

	content := message["content"]
	if contentArr, ok := content.([]interface{}); ok {
		for _, item := range contentArr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, _ := itemMap["type"].(string); itemType == "tool_result" {
					if resultContent, ok := itemMap["content"].(string); ok && resultContent != "" {
						// Use ShellFormatter for generic tool result
						formatter := NewShellFormatter(0)
						output := &ShellOutput{
							ToolName: "Tool",
							Stdout:   resultContent,
						}
						return formatter.Format(output)
					}
				}
			}
		}
	}

	return ""
}

// TurnPart represents a piece of content within a turn
type TurnPart struct {
	Type    string // "user", "diff", "assistant", "question", "tool_result"
	Content string
	Meta    string // For diffs: filename
}

// ConversationTurn represents a user message and all subsequent content until next user message.
// Includes user query, any diffs from edits, and assistant responses.
type ConversationTurn struct {
	Parts   []TurnPart // All parts in order: user, diffs, assistant text
	LineNum int
}

// ExtractUserContent extracts text from a user message
func (p *JSONLParser) ExtractUserContent(msg map[string]interface{}) string {
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return ""
	}

	content := message["content"]
	if content == nil {
		return ""
	}

	// User messages typically have string content
	if contentStr, ok := content.(string); ok {
		return contentStr
	}

	// Sometimes content might be an array (tool results or mixed)
	if contentArr, ok := content.([]interface{}); ok {
		var textParts []string

		for _, item := range contentArr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				itemType, _ := itemMap["type"].(string)

				// Only extract actual user text, not tool results
				if itemType == "text" {
					if text, ok := itemMap["text"].(string); ok && text != "" {
						textParts = append(textParts, text)
					}
				} else if itemType == "tool_result" {
					// Extract tool result content
					if toolResult, ok := itemMap["content"].(string); ok && toolResult != "" {
						textParts = append(textParts, toolResult)
					}
				}
				// Skip other types (tool_use, etc.)
			}
		}

		return strings.Join(textParts, "\n")
	}

	return ""
}

// ExtractAssistantContent extracts text from an assistant message
func (p *JSONLParser) ExtractAssistantContent(msg map[string]interface{}) string {
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return ""
	}

	content := message["content"]
	if content == nil {
		return ""
	}

	// Assistant messages have array content
	contentArr, ok := content.([]interface{})
	if !ok {
		return ""
	}

	var textParts []string
	for _, item := range contentArr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)
		if itemType == "text" {
			if text, ok := itemMap["text"].(string); ok {
				// Filter out raw XML invoke blocks
				if !strings.Contains(text, "<function_calls>") &&
					!strings.Contains(text, "<invoke") {
					textParts = append(textParts, text)
				}
			}
		}
		// tool_use is now hidden - no longer shown in assistant text
	}

	return strings.Join(textParts, "\n")
}



// CreateTurnBlock creates a Block from a ConversationTurn
// All parts (user, diff, assistant) are combined into a single scrollable view
// Parts are rendered in chronological order as they appear in the transcript
func (p *JSONLParser) CreateTurnBlock(turn *ConversationTurn, turnNumber int) Block {
	name := fmt.Sprintf("block-%d", turnNumber)

	var contentParts []string

	decorate := p.Decorate
	if decorate == nil {
		decorate = plainPart
	}

	for _, part := range turn.Parts {
		switch part.Type {
		case "user", "diff", "assistant", "question":
			contentParts = append(contentParts, decorate(part.Type, part.Content, part.Meta))

		case "tool_result":
			// Tool result output: already formatted by ShellFormatter --
			// identical on every surface, so it bypasses the decorator.
			contentParts = append(contentParts, part.Content)
		}
	}

	// Single unified content
	fullContent := strings.Join(contentParts, "\n\n")

	// Single page - no pagination
	return Block{
		Name:        name,
		Content:     fullContent,
		LineNum:     turn.LineNum,

		Pages:       []string{fullContent},
		TotalPages:  1,
		ContentType: BlockContentTranscript,
		PageTypes:   []BlockContentType{BlockContentTranscript},
		PageMeta:    []string{""},
		Data:        &TranscriptData{TurnParts: turn.Parts},
	}
}

// buildSummaryPage creates a summary page with user query, edits, and assistant response
func buildSummaryPage(userContent string, editedFiles []string, assistantContent string) string {
	var sb strings.Builder

	// User query (truncated to ~3 lines / 200 chars)
	sb.WriteString("[cyan]U:[-] ")
	userTrunc := truncateText(userContent, 200, 3)
	sb.WriteString(userTrunc)
	sb.WriteString("\n")

	// Edits section (if any)
	if len(editedFiles) > 0 {
		sb.WriteString("\n[#808080]---[-]\n")
		sb.WriteString(fmt.Sprintf("[yellow]%d edit(s):[-] ", len(editedFiles)))
		sb.WriteString(strings.Join(editedFiles, ", "))
		sb.WriteString("\n[#808080]---[-]\n")
	}

	// Assistant response (truncated)
	sb.WriteString("\n[green]A:[-] ")
	assistantTrunc := truncateText(assistantContent, 500, 10)
	sb.WriteString(assistantTrunc)

	return sb.String()
}

// truncateText truncates text to maxChars or maxLines, whichever comes first
func truncateText(text string, maxChars int, maxLines int) string {
	lines := strings.Split(text, "\n")

	// Limit lines
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		text = strings.Join(lines, "\n") + "..."
	}

	// Limit chars
	if len(text) > maxChars {
		text = text[:maxChars] + "..."
	}

	return text
}

// createDiffBlock creates a Block from a message with structuredPatch
func (p *JSONLParser) createDiffBlock(msg map[string]interface{}, diffNumber int, lineNum int) *Block {
	// Extract the diff content
	diffContent := extractStructuredPatch(msg)
	if diffContent == "" {
		return nil
	}

	// Get filename for block name
	toolUseResult, _ := msg["toolUseResult"].(map[string]interface{})
	filePath, _ := toolUseResult["filePath"].(string)
	if filePath == "" {
		filePath = fmt.Sprintf("diff-%d", diffNumber)
	} else {
		// Use just the filename, not full path
		parts := strings.Split(filePath, "/")
		filePath = parts[len(parts)-1]
	}

	// Block name shows it's a diff
	name := fmt.Sprintf("diff: %s", filePath)

	// Parse hunks for pagination
	hunks := ParseHunks(diffContent)
	numHunks := len(hunks)
	if numHunks == 0 {
		numHunks = 1
	}

	// Each hunk becomes a page - store full content, FormatDiffBlock selects hunk
	pages := make([]string, numHunks)
	pageTypes := make([]BlockContentType, numHunks)
	for i := range pages {
		pages[i] = diffContent
		pageTypes[i] = BlockContentDiff
	}

	return &Block{
		Name:        name,
		Content:     diffContent,
		LineNum:     lineNum,
		Pages:       pages,
		TotalPages:  numHunks,
		ContentType: BlockContentDiff,
		PageTypes:   pageTypes,
	}
}

