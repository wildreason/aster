package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	dashboardPort = 7700
	portMin       = 7701
	portMax       = 7799
)

// Share represents a single shared file
type Share struct {
	Port int    `json:"port"`
	Path string `json:"path"`
}

func sharesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aster", "shares.json")
}

func loadShares() ([]Share, error) {
	data, err := os.ReadFile(sharesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var shares []Share
	if err := json.Unmarshal(data, &shares); err != nil {
		return nil, err
	}
	return shares, nil
}

func saveShares(shares []Share) error {
	dir := filepath.Dir(sharesPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(shares, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sharesPath(), data, 0644)
}

func nextPort(shares []Share) (int, error) {
	used := make(map[int]bool)
	for _, s := range shares {
		used[s.Port] = true
	}
	for p := portMin; p <= portMax; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free ports in %d-%d", portMin, portMax)
}

// shortenPath replaces home dir with ~
func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// parseFileBlocks reads and parses a file into blocks for serving
func parseFileBlocks(filePath string) ([]Block, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", filePath)
	}

	ft := detectFileType(filePath)

	if ft == "img" {
		parser := &ImageParser{}
		return parser.ParseFile(filePath, false)
	}
	if ft == "vid" {
		parser := &VideoParser{}
		return parser.ParseFile(filePath, false)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fileContent := string(content)

	parser := detectParser(filePath)
	_, isJSONL := parser.(*JSONLParser)
	_, isContract := parser.(*ContractParser)
	isCSVType := detectFileType(filePath) == "csv"

	if isJSONL || isCSVType || isContract {
		return parser.Parse(fileContent), nil
	}

	contentType := BlockContentPlain
	if ft == "json" {
		contentType = BlockContentJSON
	} else if ft == "yaml" {
		contentType = BlockContentYAML
	}

	return []Block{{
		Name:        filepath.Base(filePath),
		Content:     fileContent,
		Pages:       []string{fileContent},
		TotalPages:  1,
		ContentType: contentType,
	}}, nil
}

// runShare handles the share subcommand
func runShare(args []string) {
	if len(args) == 0 {
		runShareDaemon()
		return
	}

	switch args[0] {
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: aster share add <file>")
			os.Exit(1)
		}
		runShareAdd(args[1])
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: aster share rm <port|file>")
			os.Exit(1)
		}
		runShareRm(args[1])
	case "ls":
		runShareLs()
	default:
		// Treat as file path: aster share <file> is shorthand for aster share add <file>
		runShareAdd(args[0])
	}
}

func runShareAdd(target string) {
	filePath, err := filepath.Abs(expandPath(target))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	shares, err := loadShares()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading shares: %v\n", err)
		os.Exit(1)
	}

	// Check if already shared
	for _, s := range shares {
		if s.Path == filePath {
			fmt.Fprintf(os.Stderr, "Already sharing %s on http://localhost:%d\n", shortenPath(filePath), s.Port)
			return
		}
	}

	port, err := nextPort(shares)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	shares = append(shares, Share{Port: port, Path: filePath})
	if err := saveShares(shares); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving shares: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sharing %s on http://localhost:%d\n", shortenPath(filePath), port)
}

func runShareRm(target string) {
	shares, err := loadShares()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading shares: %v\n", err)
		os.Exit(1)
	}

	targetPort, portErr := strconv.Atoi(target)
	targetPath, _ := filepath.Abs(expandPath(target))

	var kept []Share
	removed := false
	for _, s := range shares {
		if (portErr == nil && s.Port == targetPort) || s.Path == targetPath {
			fmt.Printf("Removed %s (port %d)\n", shortenPath(s.Path), s.Port)
			removed = true
			continue
		}
		kept = append(kept, s)
	}

	if !removed {
		fmt.Fprintln(os.Stderr, "No matching share found")
		os.Exit(1)
	}

	if err := saveShares(kept); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving shares: %v\n", err)
		os.Exit(1)
	}
}

func runShareLs() {
	shares, err := loadShares()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading shares: %v\n", err)
		os.Exit(1)
	}

	if len(shares) == 0 {
		fmt.Println("No active shares")
		return
	}

	fmt.Printf("%-6s  %s\n", "PORT", "FILE")
	fmt.Printf("%-6s  %s\n", "------", strings.Repeat("-", 40))
	for _, s := range shares {
		fmt.Printf("%-6d  %s\n", s.Port, shortenPath(s.Path))
	}
}

// runShareDaemon loads all shares, starts serving each, and runs the dashboard
func runShareDaemon() {
	shares, err := loadShares()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading shares: %v\n", err)
		os.Exit(1)
	}

	if len(shares) == 0 {
		fmt.Fprintln(os.Stderr, "No shares configured. Add files with: aster share add <file>")
		os.Exit(1)
	}

	var wg sync.WaitGroup

	// Start each file server
	for _, s := range shares {
		wg.Add(1)
		go func(s Share) {
			defer wg.Done()
			blocks, err := parseFileBlocks(s.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", shortenPath(s.Path), err)
				return
			}
			stopCh := make(chan struct{})
			if err := serveHTMLAsync(s.Path, blocks, s.Port, stopCh); err != nil {
				if err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "Error serving %s: %v\n", shortenPath(s.Path), err)
				}
			}
		}(s)
	}

	// Start dashboard
	wg.Add(1)
	go func() {
		defer wg.Done()
		serveDashboard(shares)
	}()

	wg.Wait()
}

// serveDashboard runs the share dashboard on port 7700
func serveDashboard(shares []Share) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, renderDashboard(shares))
	})

	addr := formatServerAddr(dashboardPort)
	fmt.Fprintf(os.Stderr, "Dashboard at http://localhost:%d\n", dashboardPort)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard error: %v\n", err)
	}
}

func renderDashboard(shares []Share) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>aster share</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: Inter, -apple-system, sans-serif;
    background: #F8FAFC;
    color: #0A1628;
    padding: 48px 24px;
    max-width: 720px;
    margin: 0 auto;
  }
  h1 {
    font-size: 20px;
    font-weight: 600;
    margin-bottom: 32px;
    color: #0A1628;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  th {
    text-align: left;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #64748B;
    padding: 8px 12px;
    border-bottom: 1px solid #E2E8F0;
  }
  td {
    padding: 10px 12px;
    font-size: 14px;
    border-bottom: 1px solid #F1F5F9;
  }
  a {
    color: #3B82F6;
    text-decoration: none;
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
  }
  a:hover { text-decoration: underline; }
  .path {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    color: #334155;
  }
  .status {
    font-size: 12px;
    color: #22C55E;
    font-weight: 500;
  }
  .empty {
    color: #94A3B8;
    font-size: 14px;
    padding: 24px 0;
  }
</style>
</head>
<body>
<h1>aster share</h1>
`)

	if len(shares) == 0 {
		sb.WriteString(`<p class="empty">No active shares. Add files with: aster share add &lt;file&gt;</p>`)
	} else {
		sb.WriteString(`<table>
<tr><th>Port</th><th>File</th><th>Status</th></tr>
`)
		for _, s := range shares {
			sb.WriteString(fmt.Sprintf(
				`<tr><td><a href="http://localhost:%d">%d</a></td><td class="path">%s</td><td class="status">live</td></tr>`+"\n",
				s.Port, s.Port, shortenPath(s.Path),
			))
		}
		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}
