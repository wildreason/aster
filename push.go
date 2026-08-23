package main

// push.go -- `aster push <file>`: publish rendered output to tunnel-artifacts
// at a STABLE address.
//
// This is the agent-native output surface. The contract a cold agent needs is
// three lines: "write the file, run `aster push <file>`, same path -> same
// URL". Everything here serves that contract:
//
//   - the address key derives deterministically from the file's place on disk
//     (aster/<project>/<relpath>), so a regenerate-and-repush loop lands on
//     the SAME artifact with zero remembered state -- no ids to capture, no
//     side files, no context tokens spent on bookkeeping;
//   - the server side is `POST /api/seed {upsert:true}` on tunnel-artifacts
//     (create-or-replace keyed on owner+path; 201 = created, 200 = replaced);
//   - the output is one line, the canonical URL, so `aster push f.html`
//     composes in a shell pipeline.
//
// What travels verbatim vs through the render pipeline:
//
//   .html/.htm    verbatim, type "html"   -- agent-generated HTML is already a
//                                            finished surface; the doc host
//                                            frames it in its own sandbox.
//   .svg          verbatim, type "svg"    -- native tunl content type.
//   .md           verbatim, type "markdown" -- the doc host renders markdown
//                                            natively (titles, readable text).
//   images        aster static render (base64 data URI) -> type "html".
//   everything    aster static render -> type "html" -- csv, diff, json,
//   else            jsonl, yaml, txt, contracts: this is where the engine
//                   earns its keep; the doc host has no renderer for these.
//
// Auth is a bearer the caller already holds (ASTER_PUSH_TOKEN / TUNL_TOKEN /
// config file) -- typically a tunl agent token minted by the `token` MCP tool.
// aster never mints or refreshes credentials itself.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pushDefaultURL is the tunnel-artifacts API base -- the host the `token`
// tool's own publish examples use (MEASURED against prod 2026-08-21, not
// read from source: tunl.wildreason.com is in the code as a default but the
// deployed service answers on tunnel.wildreason.ai). The response's `url`
// field carries the canonical artifact host (artifacts.wildreason.ai), so
// this only needs to reach the API, not name the pretty host.
const pushDefaultURL = "https://tunnel.wildreason.ai"

// pushSeedLimit mirrors tunnel-artifacts' default seed body cap
// (TUNL_SEED_MAX_BYTES, 4 MB). Checked client-side so an oversized push fails
// with a named reason instead of a bare 413.
const pushSeedLimit = 4 << 20

type pushConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func pushConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aster", "push.json")
}

// loadPushConfig resolves server + token: env first (what an agent shell
// exports), config file second (what a human sets once), default URL last.
func loadPushConfig() pushConfig {
	var cfg pushConfig
	if data, err := os.ReadFile(pushConfigPath()); err == nil {
		json.Unmarshal(data, &cfg)
	}
	if v := os.Getenv("ASTER_PUSH_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("ASTER_PUSH_TOKEN"); v != "" {
		cfg.Token = v
	} else if v := os.Getenv("TUNL_TOKEN"); v != "" && cfg.Token == "" {
		cfg.Token = v
	}
	if cfg.URL == "" {
		cfg.URL = pushDefaultURL
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return cfg
}

// pushKey derives the stable address for a file: aster/<project>/<relpath>,
// where <project> is the cwd basename and <relpath> the file's path relative
// to cwd, slash-normalized. Deterministic by construction -- the same file in
// the same project always maps to the same key, which is what makes re-push
// idempotent. A file outside cwd falls back to aster/<parent-dir>/<basename>.
func pushKey(filePath string) string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}
	// Resolve symlinks on BOTH sides before Rel, or the same file keys
	// differently by spelling (macOS: /tmp and /var are symlinks into
	// /private, so an absolute path misses a cwd that Getwd reports
	// resolved). Determinism is the whole contract here.
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = resolved
	}
	cwd, err := os.Getwd()
	if err == nil {
		if resolved, rerr := filepath.EvalSymlinks(cwd); rerr == nil {
			cwd = resolved
		}
		if rel, rerr := filepath.Rel(cwd, abs); rerr == nil && !strings.HasPrefix(rel, "..") {
			return "aster/" + filepath.Base(cwd) + "/" + filepath.ToSlash(rel)
		}
	}
	return "aster/" + filepath.Base(filepath.Dir(abs)) + "/" + filepath.Base(abs)
}

// renderForPush turns a file into (content, tunl content type). Verbatim for
// the types the doc host renders natively; through the aster static pipeline
// (self-contained HTML, assets inlined as data URIs -- sandbox-safe, no
// /asset/ routes to 404 inside an iframe) for everything else.
func renderForPush(filePath string) (string, string, error) {
	if stat, err := os.Stat(filePath); err != nil {
		return "", "", fmt.Errorf("could not find %s", filePath)
	} else if stat.Size() > maxFileSize {
		return "", "", fmt.Errorf("file too large (%d bytes, max %d)", stat.Size(), maxFileSize)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// Verbatim rails: the doc host owns rendering for these.
	switch ext {
	case ".html", ".htm", ".xhtml":
		data, err := os.ReadFile(filePath)
		return string(data), "html", err
	case ".svg":
		data, err := os.ReadFile(filePath)
		return string(data), "svg", err
	case ".md", ".markdown":
		data, err := os.ReadFile(filePath)
		return string(data), "markdown", err
	}

	ft := detectFileType(filePath)

	// Raster images and video: static parse inlines bytes as a data URI.
	if ft == "img" || ft == "vid" {
		var parser FileParser
		if ft == "img" {
			parser = &ImageParser{}
		} else {
			parser = &VideoParser{}
		}
		blocks, err := parser.ParseFile(filePath, true)
		if err != nil {
			return "", "", err
		}
		surfaceEngineWarnings(blocks)
		return RenderStaticHTMLPage(filepath.Base(filePath), blocks, false), "html", nil
	}

	// Text rail: mirror of viewTextFile's exportHTML branch (the proven
	// --html/--share path), minus the os.Exit and the HTML arm (verbatim
	// above). Kept in lockstep with main.go -- if that branch changes, this
	// one follows.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", err
	}
	fileContent := string(data)

	parser := detectParser(filePath)
	_, isJSONL := parser.(*JSONLParser)
	_, isContract := parser.(*ContractParser)
	isCSVType := ft == "csv"

	var blocks []Block
	if isJSONL || isCSVType || isContract {
		blocks = parser.Parse(fileContent)
	} else {
		contentType := BlockContentPlain
		if ft == "json" {
			contentType = BlockContentJSON
		} else if ft == "yaml" {
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
	if !isJSONL && !isCSVType && !isContract {
		blocks[0].Content = body
		blocks[0].Pages = []string{body}
	}
	return RenderStaticHTMLPage(title, blocks, showLineNumbers), "html", nil
}

// runPush handles: aster push <file> [--as <key>] [--title <title>]
func runPush(args []string) {
	var filePath, key, title string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--as" && i+1 < len(args):
			key = args[i+1]
			i++
		case args[i] == "--title" && i+1 < len(args):
			title = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			fmt.Fprintf(os.Stderr, "Error: unknown push flag %q\n", args[i])
			os.Exit(1)
		case filePath == "":
			filePath = args[i]
		default:
			fmt.Fprintf(os.Stderr, "Error: push takes one file, got %q and %q\n", filePath, args[i])
			os.Exit(1)
		}
	}
	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: aster push <file> [--as <key>] [--title <title>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Renders the file and publishes it to tunnel-artifacts at a stable")
		fmt.Fprintln(os.Stderr, "  address. Re-pushing the same file updates the same URL.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Auth: ASTER_PUSH_TOKEN or TUNL_TOKEN env, or ~/.config/aster/push.json")
		fmt.Fprintln(os.Stderr, "        {\"url\": \"...\", \"token\": \"...\"}")
		os.Exit(1)
	}
	filePath = expandPath(filePath)

	cfg := loadPushConfig()
	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Error: no push token. Set ASTER_PUSH_TOKEN or TUNL_TOKEN, or add")
		fmt.Fprintf(os.Stderr, "\"token\" to %s.\n", pushConfigPath())
		fmt.Fprintln(os.Stderr, "Agents: mint one with the `token` tool on your tunl MCP session.")
		os.Exit(1)
	}

	content, seedType, err := renderForPush(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if key == "" {
		key = pushKey(filePath)
	}

	payload := map[string]any{
		"markdown": content,
		"type":     seedType,
		"path":     key,
		"upsert":   true,
	}
	if title != "" {
		payload["title"] = title
	}
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(body) > pushSeedLimit {
		fmt.Fprintf(os.Stderr, "Error: rendered payload is %d bytes, over the %d-byte seed limit.\n",
			len(body), pushSeedLimit)
		fmt.Fprintln(os.Stderr, "Large binaries belong on the binary rail (not yet wired into push).")
		os.Exit(1)
	}

	req, err := http.NewRequest("POST", cfg.URL+"/api/seed", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: push to %s failed: %v\n", cfg.URL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		// Surface the server's own message + hint -- tunl's auth errors name
		// the next step (e.g. "mint a fresh token"), which is exactly what a
		// cold agent needs to self-recover.
		msg := fmt.Sprintf("push failed (%d)", resp.StatusCode)
		if e, ok := out["error"].(map[string]any); ok {
			if m, ok := e["message"].(string); ok && m != "" {
				msg += ": " + m
			}
			if h, ok := e["hint"].(string); ok && h != "" {
				msg += "\n" + h
			}
		}
		fmt.Fprintln(os.Stderr, "Error: "+msg)
		os.Exit(1)
	}

	url, _ := out["url"].(string)
	if url == "" {
		url = cfg.URL // shouldn't happen; keep the output non-empty
	}
	// First line is the URL alone so `aster push f.html` composes in scripts.
	fmt.Println(url)
	verb := "updated"
	if resp.StatusCode == 201 {
		verb = "created"
	}
	fmt.Fprintf(os.Stderr, "  %s  key=%s type=%s\n", verb, key, seedType)
}
