package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// Version information (injected at build time)
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// showLineNumbers enables source file line numbers in the gutter (-n flag)
var showLineNumbers bool

// servePort enables HTTP server mode (--port N flag)
var servePort int

// exportHTML enables static HTML export to stdout (--html flag)
var exportHTML bool

// shareFlag opens rendered HTML in the default browser
var shareFlag bool

// terminalFlag forces terminal TUI rendering (-t with no type argument)
var terminalFlag bool

// outputHTML prints HTML to stdout, or writes to a temp file and opens in browser if --share is set.
func outputHTML(html string) {
	if !shareFlag {
		fmt.Print(html)
		return
	}
	f, err := os.CreateTemp("", "aster-*.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	if _, err := f.WriteString(html); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing temp file: %v\n", err)
		os.Exit(1)
	}
	f.Close()
	fmt.Fprintf(os.Stderr, "Opened in your browser.\n")
	exec.Command("open", f.Name()).Start()
}

// forceType overrides content type detection (--type TYPE flag)
var forceType string

// docTitle is a page title declared by the document itself (HTML <title>).
// It outranks the filename on every rendered surface.
var docTitle string

// validTypeNames lists the content types --type accepts, in help order
var validTypeNames = []string{"md", "html", "json", "jsonl", "diff", "txt", "yaml", "csv"}

var validTypes = func() map[string]bool {
	m := make(map[string]bool, len(validTypeNames))
	for _, t := range validTypeNames {
		m[t] = true
	}
	return m
}()

// fileType defines a supported file type with its extensions
type fileType struct {
	name       string
	extensions []string
}

var fileTypes = map[string]fileType{
	"md":    {name: "markdown", extensions: []string{".md", ".markdown"}},
	"img":   {name: "image", extensions: imageExtensions},
	"vid":   {name: "video", extensions: videoExtensions},
	"txt":   {name: "text", extensions: []string{".txt", ".log"}},
	"json":  {name: "json", extensions: []string{".json"}},
	"yaml":  {name: "yaml", extensions: []string{".yaml", ".yml"}},
	"diff":  {name: "diff", extensions: []string{".diff", ".patch"}},
	"jsonl": {name: "jsonl", extensions: []string{".jsonl"}},
	"csv":   {name: "csv", extensions: []string{".csv", ".tsv"}},
	"html":  {name: "html", extensions: []string{".html", ".htm", ".xhtml"}},
}

func detectTerminalWidth() int {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			return w
		}
	}
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w, err := parsePositiveInt(cols); err == nil {
			return w
		}
	}
	return 80
}

func detectTerminalHeight() int {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if _, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && h > 0 {
			return h
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		if h, err := parsePositiveInt(lines); err == nil {
			return h
		}
	}
	return 24
}

func parsePositiveInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0, fmt.Errorf("not positive")
	}
	return n, nil
}

// detectFileType returns the type key for a file path based on extension
func detectFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	for key, ft := range fileTypes {
		for _, e := range ft.extensions {
			if ext == e {
				return key
			}
		}
	}
	return ""
}

// detectParser selects the appropriate parser based on file extension
func detectParser(filePath string) Parser {
	parsers := []Parser{
		&TodoParser{},
		&DiffParser{},
		&CsvParser{},
		&ContractParser{},
		&HTMLParser{},
		&MarkdownParser{},
		&JSONLParser{},
		&TxtParser{},
	}

	for _, parser := range parsers {
		if parser.Detect(filePath) {
			return parser
		}
	}

	return &MarkdownParser{}
}

// detectParserFromContent tries to detect parser type from content (for stdin)
func detectParserFromContent(content string) Parser {
	if DetectBlockContentType(content) == BlockContentDiff {
		return &DiffParser{}
	}

	// HTML -> converted to markdown downstream
	if isHTMLContent(content) {
		return &HTMLParser{}
	}

	// Count JSON lines to distinguish JSONL from single JSON
	lines := strings.Split(content, "\n")
	jsonLineCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var testJSON map[string]interface{}
		if err := json.Unmarshal([]byte(line), &testJSON); err == nil {
			jsonLineCount++
		} else {
			break
		}
	}
	if jsonLineCount >= 2 {
		return &JSONLParser{}
	}

	// JSON object/array -> plain text (preserves structure)
	if isJSON(content) {
		return &TxtParser{}
	}

	// YAML -> plain text (preserves structure)
	if isYAML(content) {
		return &TxtParser{}
	}

	// CSV -> table
	if isCSV(content) {
		return &CsvParser{}
	}

	return &MarkdownParser{}
}

func hasStdinData() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func readStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if line != "" {
				builder.WriteString(line)
			}
			break
		}
		if err != nil {
			return "", err
		}
		builder.WriteString(line)
	}
	return builder.String(), nil
}

func showContentSelector(content string) map[string]bool {
	types := ScanContentTypes(content)
	if len(types) == 0 {
		return map[string]bool{"user": true, "assistant": true}
	}

	fmt.Println("Scanning transcript...")
	fmt.Println()
	fmt.Println("Found content types:")

	for i, ct := range types {
		check := " "
		if ct.Enabled {
			check = "x"
		}
		fmt.Printf("  %d. [%s] %s (%d)\n", i+1, check, ct.Name, ct.Count)
	}

	fmt.Println()
	fmt.Println("Toggle: 1-9 | Confirm: Enter")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			break
		}

		for _, ch := range input {
			if ch >= '1' && ch <= '9' {
				idx := int(ch - '1')
				if idx < len(types) {
					types[idx].Enabled = !types[idx].Enabled
				}
			}
		}

		fmt.Print("\033[F")
		for range types {
			fmt.Print("\033[F")
		}
		fmt.Print("\033[F")
		fmt.Print("\033[F")

		fmt.Println("Found content types:")
		for i, ct := range types {
			check := " "
			if ct.Enabled {
				check = "x"
			}
			fmt.Printf("  %d. [%s] %s (%d)\n", i+1, check, ct.Name, ct.Count)
		}
		fmt.Println()
		fmt.Println("Toggle: 1-9 | Confirm: Enter")
		fmt.Print("> ")
	}

	filters := make(map[string]bool)
	for _, ct := range types {
		filters[ct.Name] = ct.Enabled
	}

	fmt.Println()
	return filters
}

// expandPath expands ~ and resolves the path
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

// resolveShortcut handles -, +, help, and file arguments for a given type
func resolveShortcut(arg string, exts []string) string {
	switch arg {
	case "-":
		path, err := ShowRecentPicker(exts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return path
	case "+":
		path, err := GetNewestFile(exts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Opening: %s\n", path)
		return path
	case "help":
		return ""
	default:
		return expandPath(arg)
	}
}

// viewFile routes to the correct viewer based on file type
func viewFile(filePath string) {
	// Check if path is a directory
	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		if servePort > 0 {
			serveDirectory(filePath, servePort)
		} else {
			listDirectory(filePath)
		}
		return
	}

	ft := detectFileType(filePath)

	if ft == "img" {
		AddRecent(filePath)
		if exportHTML || servePort > 0 {
			parser := &ImageParser{}
			blocks, err := parser.ParseFile(filePath, exportHTML)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if exportHTML {
				title := filepath.Base(filePath)
				outputHTML(RenderStaticHTMLPage(title, blocks, false))
				return
			}
			serveHTML(filePath, blocks, servePort)
			return
		}
		viewImage(filePath)
		return
	}

	if ft == "vid" {
		AddRecent(filePath)
		if exportHTML || servePort > 0 {
			parser := &VideoParser{}
			blocks, err := parser.ParseFile(filePath, exportHTML)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if exportHTML {
				title := filepath.Base(filePath)
				outputHTML(RenderStaticHTMLPage(title, blocks, false))
				return
			}
			serveHTML(filePath, blocks, servePort)
			return
		}
		viewVideo(filePath)
		return
	}

	viewTextFile(filePath, forceType, false)
}

// listDirectory prints a formatted table of markdown files in a directory
func listDirectory(dirPath string) {
	files, _ := filepath.Glob(filepath.Join(dirPath, "*.md"))
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No markdown files found in %s\n", dirPath)
		os.Exit(0)
	}

	type entry struct {
		title   string
		created string
		tags    string
		path    string
	}

	var entries []entry
	maxTitle := 5 // "TITLE"

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		fm, _ := ParseFrontmatter(string(content))
		title := filepath.Base(f)
		if fm.Title != "" {
			title = fm.Title
		}
		created := fm.Created
		tags := ""
		if len(fm.Tags) > 0 {
			tags = strings.Join(fm.Tags, ", ")
		}
		if len(title) > maxTitle {
			maxTitle = len(title)
		}
		entries = append(entries, entry{title: title, created: created, tags: tags, path: f})
	}

	// Sort by created desc
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].created > entries[i].created {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Print header
	fmt.Printf("%-*s  %-12s  %s\n", maxTitle, "TITLE", "CREATED", "TAGS")
	fmt.Printf("%-*s  %-12s  %s\n", maxTitle, strings.Repeat("-", maxTitle), "------------", "----")

	for _, e := range entries {
		fmt.Printf("%-*s  %-12s  %s\n", maxTitle, e.title, e.created, e.tags)
	}
}

const maxFileSize = 100 * 1024 * 1024 // 100MB

// viewTextFile reads a file and renders it in the TUI
func viewTextFile(filePath string, forceType string, follow bool) {
	if stat, err := os.Stat(filePath); err == nil && stat.Size() > maxFileSize {
		fmt.Fprintf(os.Stderr, "Error: file too large (%d bytes, max %d)\n", stat.Size(), maxFileSize)
		os.Exit(1)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not find %s\n", filePath)
		os.Exit(1)
	}
	fileContent := string(content)
	termWidth := detectTerminalWidth()

	AddRecent(filePath)

	var parser Parser
	var isJSONL bool

	if forceType != "" {
		switch forceType {
		case "md":
			parser = &MarkdownParser{}
		case "jsonl":
			parser = &JSONLParser{}
			isJSONL = true
		case "diff":
			parser = &DiffParser{}
		case "json":
			parser = &TodoParser{}
		case "txt":
			parser = &TxtParser{}
		case "yaml":
			parser = &TxtParser{}
		case "csv":
			parser = &CsvParser{}
		case "html":
			parser = &HTMLParser{}
		}
	} else {
		parser = detectParser(filePath)
		_, isJSONL = parser.(*JSONLParser)
	}

	// HTML is converted to markdown before Parse, so every downstream surface
	// (terminal, --port, --html, --share) renders it through the same pipeline
	if _, isHTML := parser.(*HTMLParser); isHTML {
		fileContent, docTitle = HTMLToMarkdown(fileContent)
		parser = &MarkdownParser{}
	}

	if follow && servePort == 0 {
		runFollowMode(filePath, fileContent, isJSONL, termWidth, "auto", BorderNone)
		return
	}

	_, isContract := parser.(*ContractParser)

	// Static HTML export
	if exportHTML {
		isCSVType := forceType == "csv" || detectFileType(filePath) == "csv"
		var blocks []Block
		if isJSONL || isCSVType || isContract {
			blocks = parser.Parse(fileContent)
		} else {
			contentType := BlockContentPlain
			if forceType == "json" || detectFileType(filePath) == "json" {
				contentType = BlockContentJSON
			} else if forceType == "yaml" || detectFileType(filePath) == "yaml" {
				contentType = BlockContentYAML
			}
			blocks = []Block{{
				Name:        filepath.Base(filePath),
				Content:     fileContent,
				Pages:       []string{fileContent},
				TotalPages:  1,
				ContentType: contentType,
			}}
		}
		title := filepath.Base(filePath)
		fm, body := ParseFrontmatter(fileContent)
		if fm.Title != "" {
			title = fm.Title
		}
		if docTitle != "" {
			title = docTitle
		}
		if !isJSONL && !isCSVType && !isContract {
			blocks[0].Content = body
			blocks[0].Pages = []string{body}
		}
		outputHTML(RenderStaticHTMLPage(title, blocks, showLineNumbers))
		return
	}

	// For --port mode, render as single block so HTML formatter handles headings natively
	if servePort > 0 {
		isCSVType := forceType == "csv" || detectFileType(filePath) == "csv"
		var blocks []Block
		if isJSONL {
			jsonlParser := &JSONLParser{}
			filters := showContentSelector(fileContent)
			jsonlParser.Filters = filters
			blocks = jsonlParser.Parse(fileContent)
		} else if isCSVType || isContract {
			blocks = parser.Parse(fileContent)
		} else {
			// Single block: let HTML render h1/h2/h3 directly instead of splitting by headers
			contentType := BlockContentPlain
			if forceType == "json" || detectFileType(filePath) == "json" {
				contentType = BlockContentJSON
			} else if forceType == "yaml" || detectFileType(filePath) == "yaml" {
				contentType = BlockContentYAML
			}
			blocks = []Block{{
				Name:        filepath.Base(filePath),
				Content:     fileContent,
				Pages:       []string{fileContent},
				TotalPages:  1,
				ContentType: contentType,
			}}
		}
		serveHTML(filePath, blocks, servePort)
		return
	}

	var blocks []Block
	if isJSONL {
		jsonlParser := &JSONLParser{}
		filters := showContentSelector(fileContent)
		jsonlParser.Filters = filters
		blocks = jsonlParser.Parse(fileContent)
	} else if mdParser, ok := parser.(*MarkdownParser); ok {
		termHeight := detectTerminalHeight()
		blocks = mdParser.ParseContinuous(fileContent, termHeight)
	} else {
		blocks = parser.Parse(fileContent)
	}

	runReaderMode(blocks, filePath, termWidth, "auto", BorderNone)
}

// viewStdinContent renders stdin content
func viewStdinContent(content string, forceType string) {
	termWidth := detectTerminalWidth()

	var parser Parser
	var isJSONL bool

	if forceType != "" {
		switch forceType {
		case "md":
			parser = &MarkdownParser{}
		case "jsonl":
			parser = &JSONLParser{}
			isJSONL = true
		case "diff":
			parser = &DiffParser{}
		case "json":
			parser = &TodoParser{}
		case "txt":
			parser = &TxtParser{}
		case "yaml":
			parser = &TxtParser{}
		case "csv":
			parser = &CsvParser{}
		case "html":
			parser = &HTMLParser{}
		default:
			parser = &MarkdownParser{}
		}
	} else {
		parser = detectParserFromContent(content)
		_, isJSONL = parser.(*JSONLParser)
	}

	if _, isHTML := parser.(*HTMLParser); isHTML {
		content, docTitle = HTMLToMarkdown(content)
		parser = &MarkdownParser{}
	}

	var blocks []Block
	if isJSONL {
		jsonlParser := &JSONLParser{}
		filters := showContentSelector(content)
		jsonlParser.Filters = filters
		blocks = jsonlParser.Parse(content)
	} else if mdParser, ok := parser.(*MarkdownParser); ok {
		termHeight := detectTerminalHeight()
		blocks = mdParser.ParseContinuous(content, termHeight)
	} else {
		blocks = parser.Parse(content)
	}

	// Static HTML export
	if exportHTML {
		title := "stdin"
		if docTitle != "" {
			title = docTitle
		}
		outputHTML(RenderStaticHTMLPage(title, blocks, showLineNumbers))
		return
	}

	// Web mode: serve as HTML
	if servePort > 0 {
		serveHTML("stdin", blocks, servePort)
		return
	}

	runReaderMode(blocks, "stdin", termWidth, "auto", BorderNone)
}

func printUsage() {
	w := os.Stderr
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Read any file, rendered in your browser.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  aster <file>          Open file in browser (default)")
	fmt.Fprintln(w, "  aster <file> -t       View file in terminal")
	fmt.Fprintln(w, "  aster pick            Pick from recent files")
	fmt.Fprintln(w, "  aster latest          Open newest file in current directory")
	fmt.Fprintln(w, "  aster push <file>     Publish rendered output to a stable web address")
	fmt.Fprintln(w, "                        (re-push updates the same URL; --as KEY, --title T)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -t, --terminal        Render in terminal instead of browser")
	fmt.Fprintln(w, "  --type TYPE           Force content type (md, html, json, jsonl, diff, txt, yaml, csv)")
	fmt.Fprintln(w, "  -n                    Show source file line numbers")
	fmt.Fprintln(w, "  --port N              Serve rendered HTML on localhost:N")
	fmt.Fprintln(w, "  --html                Export self-contained HTML to stdout")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Supported formats:")
	fmt.Fprintln(w, "  Markdown        .md .markdown")
	fmt.Fprintln(w, "  HTML            .html .htm .xhtml")
	fmt.Fprintln(w, "  Plain text      .txt .log")
	fmt.Fprintln(w, "  Unified diffs   .diff .patch")
	fmt.Fprintln(w, "  JSON            .json")
	fmt.Fprintln(w, "  Transcripts     .jsonl")
	fmt.Fprintln(w, "  CSV/TSV         .csv .tsv")
	fmt.Fprintln(w, "  Images          .png .jpg .gif .webp .bmp .svg")
	fmt.Fprintln(w, "  Video           .mp4 .webm .mov .mkv")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Navigation:")
	fmt.Fprintln(w, "  j / k             Scroll down / up")
	fmt.Fprintln(w, "  d / u             Half-page down / up")
	fmt.Fprintln(w, "  g / G             Top / bottom")
	fmt.Fprintln(w, "  PgDn / PgUp       Full page down / up")
	fmt.Fprintln(w, "  q                 Quit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Piping:")
	fmt.Fprintln(w, "  git diff HEAD~3 | aster           Auto-detect and render diff")
	fmt.Fprintln(w, "  curl api.com/data | aster         Auto-detect JSON")
	fmt.Fprintln(w, "  cat log.jsonl | aster --type jsonl Force content type")
	fmt.Fprintln(w, "  curl example.com | aster          Auto-detect and render HTML")
	fmt.Fprintln(w, "  aster file.md | head -20          Passthrough when piped out")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  aster readme.md               View markdown with colors and tables")
	fmt.Fprintln(w, "  aster page.html               Render HTML as readable text")
	fmt.Fprintln(w, "  aster screenshot.png          Render image inline")
	fmt.Fprintln(w, "  aster changes.patch           View diff with syntax highlighting")
	fmt.Fprintln(w, "  aster pick                    Choose from recently viewed files")
	fmt.Fprintln(w, "  aster latest                  Open the newest file in cwd")
	fmt.Fprintln(w, "  aster file.md --port 3000     Serve rendered HTML on localhost")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Images require chafa (brew install chafa).")
	fmt.Fprintln(w)
}

func main() {
	// Parse flags early (before other arg processing)
	var cleanArgs []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-n":
			showLineNumbers = true

		case args[i] == "--html":
			exportHTML = true

		case args[i] == "--share":
			shareFlag = true

		case args[i] == "--port" && i+1 < len(args):
			p, err := parsePositiveInt(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: --port requires a positive integer\n")
				os.Exit(1)
			}
			servePort = p
			i++ // skip the port number

		case args[i] == "-t" || args[i] == "--terminal":
			// -t selects the output surface and consumes nothing. A type name
			// after it (aster -t md file.md) stays in the args, where the
			// subcommand dispatcher picks it up.
			terminalFlag = true

		case args[i] == "--type" || strings.HasPrefix(args[i], "--type="):
			value := ""
			if after, ok := strings.CutPrefix(args[i], "--type="); ok {
				value = after
			} else if i+1 < len(args) {
				value = args[i+1]
				i++ // skip the type value
			}
			if !validTypes[value] {
				fmt.Fprintf(os.Stderr, "Error: --type expects one of: %s\n", strings.Join(validTypeNames, ", "))
				os.Exit(1)
			}
			forceType = value

		default:
			cleanArgs = append(cleanArgs, args[i])
		}
	}
	os.Args = append([]string{os.Args[0]}, cleanArgs...)

	// "aster file.md -t md" used to force a content type. -t no longer takes a
	// value and nothing else reads a trailing type name, so say so rather than
	// ignoring it.
	for _, arg := range cleanArgs[min(1, len(cleanArgs)):] {
		if validTypes[arg] {
			fmt.Fprintf(os.Stderr, "Error: stray argument %q. Use --type %s to force content type.\n", arg, arg)
			os.Exit(1)
		}
	}

	if shareFlag {
		exportHTML = true
	}

	// Default to browser if no output mode specified and not terminal mode
	if !terminalFlag && !exportHTML && !shareFlag && servePort == 0 {
		shareFlag = true
		exportHTML = true
	}

	// Check for subcommand or shortcut as first arg
	if len(os.Args) >= 2 {
		first := os.Args[1]

		switch {
		case first == "help" || first == "--help":
			printUsage()
			return
		case first == "--version":
			fmt.Printf("aster %s\n", Version)
			fmt.Printf("  commit: %s\n", Commit)
			fmt.Printf("  built:  %s\n", Date)
			return
		case first == "pick" || first == "p" || first == "-":
			TrackUsage("pick")
			path, err := ShowRecentPicker(nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			viewFile(path)
			return
		case first == "latest" || first == "l" || first == "+":
			TrackUsage("latest")
			path, err := GetNewestFile(nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Opening: %s\n", path)
			viewFile(path)
			return
		case first == "push":
			TrackUsage("push")
			runPush(os.Args[2:])
			return
		}

		// Subcommand mode: aster <type> [file|-|+]
		if ft, ok := fileTypes[first]; ok {
			TrackUsage(first)
			runSubcommand(first, ft, os.Args[2:])
			return
		}

		// Hidden flag: -f <file>
		if first == "-f" && len(os.Args) >= 3 {
			filePath := expandPath(os.Args[2])
			viewTextFile(filePath, "", true)
			return
		}

		// Default: treat as file path
		TrackUsage("view")
		filePath := expandPath(first)

		viewFile(filePath)
		return
	}

	// No args: try stdin
	TrackUsage("stdin")
	if hasStdinData() {
		content, err := readStdin()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		viewStdinContent(content, forceType)
		return
	}

	fmt.Fprintln(os.Stderr, "No file provided. Run 'aster help' for usage.")
	os.Exit(1)
}

// runSubcommand handles: aster <type> [file|-|+]
func runSubcommand(typeName string, ft fileType, args []string) {
	if len(args) == 0 {
		if hasStdinData() {
			content, err := readStdin()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			viewStdinContent(content, typeName)
			return
		}
		fmt.Fprintf(os.Stderr, "Usage: aster %s [file | - | +]\n\n", typeName)
		fmt.Fprintf(os.Stderr, "  aster %s <file>   View %s file\n", typeName, ft.name)
		fmt.Fprintf(os.Stderr, "  aster %s -        Pick from recent %s files\n", typeName, ft.name)
		fmt.Fprintf(os.Stderr, "  aster %s +        Open newest %s in cwd\n", typeName, ft.name)
		os.Exit(1)
	}

	// Handle -f as hidden flag within subcommand
	follow := false
	target := args[0]
	if target == "-f" && len(args) >= 2 {
		follow = true
		target = args[1]
	}

	filePath := resolveShortcut(target, ft.extensions)
	if filePath == "" {
		fmt.Fprintf(os.Stderr, "Usage: aster %s [file | - | +]\n", typeName)
		os.Exit(0)
	}

	if typeName == "img" {
		AddRecent(filePath)
		if exportHTML || servePort > 0 {
			parser := &ImageParser{}
			blocks, err := parser.ParseFile(filePath, exportHTML)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if exportHTML {
				outputHTML(RenderStaticHTMLPage(filepath.Base(filePath), blocks, false))
			} else {
				serveHTML(filePath, blocks, servePort)
			}
		} else {
			viewImage(filePath)
		}
	} else if typeName == "vid" {
		AddRecent(filePath)
		if exportHTML || servePort > 0 {
			parser := &VideoParser{}
			blocks, err := parser.ParseFile(filePath, exportHTML)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if exportHTML {
				outputHTML(RenderStaticHTMLPage(filepath.Base(filePath), blocks, false))
			} else {
				serveHTML(filePath, blocks, servePort)
			}
		} else {
			viewVideo(filePath)
		}
	} else {
		viewTextFile(filePath, typeName, follow)
	}
}
