package main

// Golden-parity harness for the Phase 1 engine extraction.
//
// PURPOSE: the extraction (engine/ package, exported API, module rename) must
// not change ONE BYTE of rendered output. These goldens are banked from the
// pre-refactor tree; every commit of the refactor must keep them green. If a
// diff here is ever intentional, regenerate with -update and say why in the
// commit -- an unexplained golden change during a "mechanical" refactor is
// the exact defect class this file exists to catch.
//
// The render path mirrored here is viewTextFile's exportHTML branch plus the
// image branch of viewFile -- i.e. what `aster <file> --html` (and push's
// rendered rail) actually does. Fixtures live in testdata/golden/, one per
// content type the engine claims to support.
//
// Run `go test -run TestGolden -update` to (re)bank.

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files from current output")

// renderForGolden reproduces the CLI --html path per file type. Kept in
// lockstep with viewTextFile/viewFile (pre-refactor) and the engine API
// (post-refactor); byte parity between the two states is the test.
func renderForGolden(t *testing.T, path string) string {
	t.Helper()

	ft := detectFileType(path)
	if ft == "img" {
		parser := &ImageParser{}
		blocks, err := parser.ParseFile(path, true)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return RenderStaticHTMLPage(filepath.Base(path), blocks, false)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	fileContent := string(data)

	parser := detectParser(path)
	_, isJSONL := parser.(*JSONLParser)
	_, isContract := parser.(*ContractParser)
	isCSVType := ft == "csv"
	title := filepath.Base(path)

	// HTML converts to markdown before Parse, exactly as viewTextFile does.
	if _, isHTML := parser.(*HTMLParser); isHTML {
		var htmlTitle string
		fileContent, htmlTitle = HTMLToMarkdown(fileContent)
		parser = &MarkdownParser{}
		if htmlTitle != "" {
			title = htmlTitle
		}
	}

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
			Name:        filepath.Base(path),
			Content:     fileContent,
			Pages:       []string{fileContent},
			TotalPages:  1,
			ContentType: contentType,
		}}
	}
	fm, body := ParseFrontmatter(fileContent)
	if fm.Title != "" {
		title = fm.Title
	}
	if !isJSONL && !isCSVType && !isContract {
		blocks[0].Content = body
		blocks[0].Pages = []string{body}
	}
	return RenderStaticHTMLPage(title, blocks, false)
}

func TestGoldenParity(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/golden/sample.*")
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no golden fixtures found: %v", err)
	}

	rendered := 0
	for _, fx := range fixtures {
		if strings.HasSuffix(fx, ".golden.html") {
			continue
		}
		fx := fx
		t.Run(filepath.Base(fx), func(t *testing.T) {
			got := renderForGolden(t, fx)
			goldenPath := fx + ".golden.html"

			if *updateGolden {
				if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("no golden for %s -- run `go test -run TestGolden -update` on the KNOWN-GOOD tree first", fx)
			}
			if got != string(want) {
				// Point at the first divergence rather than dumping 100KB.
				i := 0
				for i < len(got) && i < len(want) && got[i] == want[i] {
					i++
				}
				lo := i - 60
				if lo < 0 {
					lo = 0
				}
				hiG, hiW := i+120, i+120
				if hiG > len(got) {
					hiG = len(got)
				}
				if hiW > len(want) {
					hiW = len(want)
				}
				t.Errorf("byte divergence at offset %d\n got: ...%q...\nwant: ...%q...", i, got[lo:hiG], string(want)[lo:hiW])
			}
		})
		rendered++
	}
	if !*updateGolden && rendered < 11 {
		t.Errorf("only %d fixtures rendered; the golden set must cover every content type (11)", rendered)
	}
}
