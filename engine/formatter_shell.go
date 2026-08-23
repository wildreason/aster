package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// ShellOutput represents parsed shell/tool output data
type ShellOutput struct {
	ToolName  string   // "Bash", "Read", "Glob", "Grep"
	Command   string   // Command that was run or pattern
	Stdout    string   // Standard output
	Stderr    string   // Standard error
	FilePath  string   // For Read tool: file path
	FileCount int      // For Glob/Grep: number of files
	FileList  []string // For Glob/Grep: list of files
}

// ShellFormatter renders shell command output with proper styling
type ShellFormatter struct {
	Width           int
	MaxLines        int // 0 = unlimited, default 100
	TruncationLimit int // Chars before truncation, default 5000
}

// Color constants for shell formatting (tview tags)
const (
	shellHeaderColor    = "[#179299:-:b]" // Bold teal for tool:command header
	shellStdoutColor    = "[-]"           // Default for stdout
	shellStderrColor    = "[#E05252]"     // Coral for stderr
	shellFilePathColor  = "[#1e66f5]"     // Blue for file paths
	shellTruncatedColor = "[#808080]"     // Gray for truncation notice
	shellResetColor     = "[-]"
)

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// NewShellFormatter creates a formatter with default settings
func NewShellFormatter(width int) *ShellFormatter {
	return &ShellFormatter{
		Width:           width,
		MaxLines:        100,
		TruncationLimit: 5000,
	}
}

// StripANSI removes all ANSI escape codes from content
func (f *ShellFormatter) StripANSI(content string) string {
	return ansiRegex.ReplaceAllString(content, "")
}

// Format renders ShellOutput with visual styling for display
// Shows only Tool: command/path header (compact mode)
func (f *ShellFormatter) Format(output *ShellOutput) string {
	if output == nil {
		return ""
	}

	// Just show the header line - no content body
	return f.formatHeader(output)
}

// formatHeader creates the tool:command header line
func (f *ShellFormatter) formatHeader(output *ShellOutput) string {
	if output.ToolName == "" {
		return ""
	}

	var header string
	switch output.ToolName {
	case "Bash":
		// For Bash, show first line of output as preview (command not available in toolUseResult)
		preview := f.getFirstLine(f.StripANSI(output.Stdout))
		if preview != "" {
			header = fmt.Sprintf("%sBash:%s %s", shellHeaderColor, shellResetColor, preview)
		} else {
			header = fmt.Sprintf("%sBash:%s (no output)", shellHeaderColor, shellResetColor)
		}
	case "Read":
		if output.FilePath != "" {
			header = fmt.Sprintf("%sRead:%s %s%s%s",
				shellHeaderColor, shellResetColor,
				shellFilePathColor, output.FilePath, shellResetColor)
		} else {
			header = fmt.Sprintf("%sRead:%s", shellHeaderColor, shellResetColor)
		}
	case "Glob", "Grep":
		if output.Command != "" {
			header = fmt.Sprintf("%s%s:%s %s",
				shellHeaderColor, output.ToolName, shellResetColor, output.Command)
		} else {
			header = fmt.Sprintf("%s%s:%s", shellHeaderColor, output.ToolName, shellResetColor)
		}
		if output.FileCount > 0 {
			header += fmt.Sprintf(" %s(%d files)%s", shellTruncatedColor, output.FileCount, shellResetColor)
		}
	default:
		if output.Command != "" {
			header = fmt.Sprintf("%s%s:%s %s",
				shellHeaderColor, output.ToolName, shellResetColor, output.Command)
		} else {
			header = fmt.Sprintf("%s%s:%s", shellHeaderColor, output.ToolName, shellResetColor)
		}
	}

	return header
}

// getFirstLine extracts and truncates the first non-empty line
func (f *ShellFormatter) getFirstLine(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Truncate long lines
			if len(line) > 60 {
				return line[:60] + "..."
			}
			return line
		}
	}
	return ""
}

// ParseToolResult converts toolUseResult JSON map to ShellOutput
// Returns nil for tool results that should be skipped (Edit with diff, Todo, etc.)
func ParseToolResult(toolUseResult map[string]interface{}) *ShellOutput {
	if toolUseResult == nil {
		return nil
	}

	// Skip Edit tool results (handled as diff separately)
	if _, hasStructuredPatch := toolUseResult["structuredPatch"]; hasStructuredPatch {
		return nil
	}

	// Skip Todo tool results (not useful to display)
	if _, hasTodos := toolUseResult["newTodos"]; hasTodos {
		return nil
	}

	output := &ShellOutput{}

	// Check for Bash tool (stdout/stderr)
	if stdout, ok := toolUseResult["stdout"].(string); ok {
		output.ToolName = "Bash"
		output.Stdout = stdout
		if stderr, ok := toolUseResult["stderr"].(string); ok {
			output.Stderr = stderr
		}
		return output
	}

	// Check for Read tool (file content)
	if file, ok := toolUseResult["file"].(map[string]interface{}); ok {
		output.ToolName = "Read"
		if filePath, ok := file["filePath"].(string); ok {
			output.FilePath = filePath
		}
		if content, ok := file["content"].(string); ok {
			output.Stdout = content
		}
		return output
	}

	// Check for Glob/Grep tool (filenames)
	if filenames, ok := toolUseResult["filenames"].([]interface{}); ok {
		output.ToolName = "Glob"
		output.FileCount = len(filenames)
		output.FileList = make([]string, 0, len(filenames))
		for _, f := range filenames {
			if fname, ok := f.(string); ok {
				output.FileList = append(output.FileList, fname)
			}
		}
		if numFiles, ok := toolUseResult["numFiles"].(float64); ok {
			output.FileCount = int(numFiles)
		}
		return output
	}

	// Check for filePath (Edit tool result without structuredPatch - already filtered above)
	if filePath, ok := toolUseResult["filePath"].(string); ok {
		output.ToolName = "Edit"
		output.FilePath = filePath
		return output
	}

	return nil
}
