package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// formatServerAddr returns the listen address for localhost binding
func formatServerAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// sseClient represents a connected SSE client
type sseClient struct {
	ch chan struct{}
}

// sseBroadcaster manages SSE client subscriptions
type sseBroadcaster struct {
	mu      sync.Mutex
	clients map[*sseClient]struct{}
}

func newSSEBroadcaster() *sseBroadcaster {
	return &sseBroadcaster{
		clients: make(map[*sseClient]struct{}),
	}
}

func (b *sseBroadcaster) subscribe() *sseClient {
	c := &sseClient{ch: make(chan struct{}, 1)}
	b.mu.Lock()
	b.clients[c] = struct{}{}
	b.mu.Unlock()
	return c
}

func (b *sseBroadcaster) unsubscribe(c *sseClient) {
	b.mu.Lock()
	delete(b.clients, c)
	b.mu.Unlock()
}

func (b *sseBroadcaster) notify() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.clients {
		select {
		case c.ch <- struct{}{}:
		default:
		}
	}
}

// imageMIME returns the MIME type for an image extension
func imageMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// videoMIME returns the MIME type for a video extension
func videoMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}

// serveHTMLAsync starts an HTTP server on the given port and returns an error
// instead of calling os.Exit. The stopCh can be closed to signal shutdown.
func serveHTMLAsync(filePath string, blocks []Block, port int, stopCh <-chan struct{}) error {
	var (
		mu          sync.RWMutex
		currentHTML string
		broadcaster = newSSEBroadcaster()
		title       = filepath.Base(filePath)
	)

	// Extract frontmatter title if available (for single-block markdown)
	if len(blocks) == 1 {
		fm, body := ParseFrontmatter(blocks[0].Content)
		if fm.Title != "" {
			title = fm.Title
		}
		blocks[0].Content = body
		blocks[0].Pages = []string{body}
	}

	// A title the document declares outranks the filename
	if docTitle != "" {
		title = docTitle
	}

	// Initial render
	currentHTML = RenderHTMLPage(title, blocks, showLineNumbers)

	// File watcher: re-parse + re-render on change, notify SSE clients
	if filePath != "" && filePath != "stdin" {
		watchStop := make(chan struct{})
		go func() {
			select {
			case <-stopCh:
				close(watchStop)
			case <-watchStop:
			}
		}()

		singleBlock := len(blocks) == 1
		ct := BlockContentPlain
		if singleBlock {
			ct = blocks[0].ContentType
		}
		go watchAndRerender(filePath, title, singleBlock, ct, &mu, &currentHTML, broadcaster, watchStop)
	}

	mux := http.NewServeMux()

	// Register asset routes for binary content (images, video)
	for _, block := range blocks {
		switch block.ContentType {
		case BlockContentImage:
			if imgData, ok := block.Data.(*ImageData); ok && !imgData.Inline {
				assetPath := imgData.Src
				assetMIME := imgData.MIME
				mux.HandleFunc("/asset/"+filepath.Base(imgData.Alt), func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", assetMIME)
					w.Header().Set("Cache-Control", "no-cache")
					http.ServeFile(w, r, assetPath)
				})
			}
		case BlockContentVideo:
			if vidData, ok := block.Data.(*VideoData); ok && !vidData.Inline {
				assetPath := vidData.Src
				assetMIME := vidData.MIME
				mux.HandleFunc("/asset/"+filepath.Base(block.Name), func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", assetMIME)
					w.Header().Set("Cache-Control", "no-cache")
					http.ServeFile(w, r, assetPath)
				})
			}
		}
	}

	// GET / -- serve rendered HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		mu.RLock()
		page := currentHTML
		mu.RUnlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})

	// GET /events -- SSE endpoint
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		client := broadcaster.subscribe()
		defer broadcaster.unsubscribe(client)

		for {
			select {
			case <-client.ch:
				fmt.Fprintf(w, "data: reload\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	server := &http.Server{
		Addr:    formatServerAddr(port),
		Handler: mux,
	}

	// Shutdown when stopCh is closed
	go func() {
		<-stopCh
		server.Close()
	}()

	fmt.Fprintf(os.Stderr, "Serving at http://localhost:%d\n", port)
	return server.ListenAndServe()
}

// serveHTML starts an HTTP server serving the rendered file (blocking, exits on error)
func serveHTML(filePath string, blocks []Block, port int) {
	stopCh := make(chan struct{})
	if err := serveHTMLAsync(filePath, blocks, port, stopCh); err != nil {
		if err == http.ErrServerClosed {
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// docEntry holds a cached document for directory mode
type docEntry struct {
	slug     string
	fm       Frontmatter
	html     string
	modTime  time.Time
}

// serveDirectory starts an HTTP server listing all markdown files in dirPath
func serveDirectory(dirPath string, port int) {
	var (
		mu          sync.RWMutex
		cache       = make(map[string]*docEntry) // slug -> entry
		indexHTML   string
		broadcaster = newSSEBroadcaster()
		dirName     = filepath.Base(dirPath)
	)

	// Initial scan
	scanDirectory(dirPath, cache)
	indexHTML = renderIndex(dirName, cache)

	mux := http.NewServeMux()

	// GET /events -- SSE endpoint
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		client := broadcaster.subscribe()
		defer broadcaster.unsubscribe(client)

		for {
			select {
			case <-client.ch:
				fmt.Fprintf(w, "data: reload\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// GET / and GET /{slug}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()

		path := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if path == "" {
			fmt.Fprint(w, indexHTML)
			return
		}

		entry, ok := cache[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, entry.html)
	})

	// Directory watcher
	go watchDirectory(dirPath, dirName, &mu, cache, &indexHTML, broadcaster)

	addr := formatServerAddr(port)
	fmt.Fprintf(os.Stderr, "Serving at http://localhost:%d\n", port)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// slugFromPath converts a filename to a URL slug
func slugFromPath(filePath string) string {
	name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	return name
}

// scanDirectory reads all .md files and populates the cache
func scanDirectory(dirPath string, cache map[string]*docEntry) {
	files, _ := filepath.Glob(filepath.Join(dirPath, "*.md"))
	for _, f := range files {
		loadDocEntry(f, cache)
	}
}

// loadDocEntry reads a single file and adds/updates it in the cache
func loadDocEntry(filePath string, cache map[string]*docEntry) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		return
	}

	slug := slugFromPath(filePath)
	fm, body := ParseFrontmatter(string(content))

	title := slug
	if fm.Title != "" {
		title = fm.Title
	}

	blocks := []Block{{
		Name:        title,
		Content:     body,
		Pages:       []string{body},
		TotalPages:  1,
		ContentType: BlockContentPlain,
	}}
	rendered := RenderHTMLPage(title, blocks, false)

	cache[slug] = &docEntry{
		slug:    slug,
		fm:      fm,
		html:    rendered,
		modTime: stat.ModTime(),
	}
}

// renderIndex builds the index HTML from current cache
func renderIndex(dirName string, cache map[string]*docEntry) string {
	docs := make([]DocMeta, 0, len(cache))
	for _, entry := range cache {
		title := entry.slug
		if entry.fm.Title != "" {
			title = entry.fm.Title
		}
		docs = append(docs, DocMeta{
			Slug:    entry.slug,
			Title:   title,
			Created: entry.fm.Created,
			Tags:    entry.fm.Tags,
			ModTime: entry.modTime,
		})
	}
	// Sort by created date desc, then by title
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Created != docs[j].Created {
			return docs[i].Created > docs[j].Created
		}
		return docs[i].Title < docs[j].Title
	})
	return RenderIndexPage(dirName, docs)
}

// watchDirectory polls for file changes in dirPath and updates cache
func watchDirectory(dirPath string, dirName string, mu *sync.RWMutex, cache map[string]*docEntry, indexHTML *string, broadcaster *sseBroadcaster) {
	for {
		time.Sleep(500 * time.Millisecond)

		files, _ := filepath.Glob(filepath.Join(dirPath, "*.md"))
		currentSlugs := make(map[string]string) // slug -> filepath
		for _, f := range files {
			currentSlugs[slugFromPath(f)] = f
		}

		changed := false

		mu.RLock()
		// Check for new or modified files
		for slug, fpath := range currentSlugs {
			stat, err := os.Stat(fpath)
			if err != nil {
				continue
			}
			entry, exists := cache[slug]
			if !exists || stat.ModTime().After(entry.modTime) {
				changed = true
				break
			}
		}
		// Check for deleted files
		for slug := range cache {
			if _, exists := currentSlugs[slug]; !exists {
				changed = true
				break
			}
		}
		mu.RUnlock()

		if !changed {
			continue
		}

		mu.Lock()
		// Remove deleted entries
		for slug := range cache {
			if _, exists := currentSlugs[slug]; !exists {
				delete(cache, slug)
			}
		}
		// Add/update entries
		for slug, fpath := range currentSlugs {
			stat, err := os.Stat(fpath)
			if err != nil {
				continue
			}
			entry, exists := cache[slug]
			if !exists || stat.ModTime().After(entry.modTime) {
				loadDocEntry(fpath, cache)
			}
		}
		*indexHTML = renderIndex(dirName, cache)
		mu.Unlock()

		broadcaster.notify()
	}
}

// watchAndRerender polls the file for changes, re-parses, re-renders HTML, and notifies SSE clients
func watchAndRerender(filePath string, title string, singleBlock bool, contentType BlockContentType, mu *sync.RWMutex, currentHTML *string, broadcaster *sseBroadcaster, stopCh <-chan struct{}) {
	parser := detectParser(filePath)
	var lastModTime time.Time

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		time.Sleep(500 * time.Millisecond)

		stat, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if stat.ModTime().After(lastModTime) {
			lastModTime = stat.ModTime()

			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var blocks []Block
			renderTitle := title
			var rendered string
			if singleBlock {
				bodyStr := string(content)
				fm, body := ParseFrontmatter(bodyStr)
				if fm.Title != "" {
					renderTitle = fm.Title
				}
				bodyStr = body

				blocks = []Block{{
					Name:        renderTitle,
					Content:     bodyStr,
					Pages:       []string{bodyStr},
					TotalPages:  1,
					ContentType: contentType,
				}}
				rendered = RenderHTMLPage(renderTitle, blocks, showLineNumbers)
			} else {
				blocks = parser.Parse(string(content))
				if len(blocks) == 0 {
					continue
				}
				rendered = RenderHTMLPage(renderTitle, blocks, showLineNumbers)
			}

			mu.Lock()
			*currentHTML = rendered
			mu.Unlock()

			broadcaster.notify()
		}
	}
}
