package engine

// Purity is enforced, not promised: the engine is a library that hosts
// (tunnel-artifacts among them) import into a server process. It must never
// exit or write to the process's streams, never open network listeners, and
// never depend on a terminal. These tests red on the DIFF that adds a
// violation, so the property survives review turnover.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var forbiddenImports = map[string]string{
	"net/http":                 "an engine renders bytes; serving them is the host's job",
	"github.com/rivo/tview":    "terminal UI belongs to the CLI",
	"github.com/gdamore/tcell": "terminal UI belongs to the CLI",
	"golang.org/x/term":        "the engine must not probe a terminal",
}

func enginesSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, name)
		}
	}
	if len(files) < 10 {
		t.Fatalf("only %d engine source files found -- glob broken?", len(files))
	}
	return files
}

func TestEngineImportsNoForbiddenPackages(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range enginesSourceFiles(t) {
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if why, bad := forbiddenImports[path]; bad {
				t.Errorf("%s imports %s -- %s", f, path, why)
			}
		}
	}
}

func TestEngineNeverExitsOrWritesStreams(t *testing.T) {
	// Source-text scan (not AST) so aliased/indirect forms still surface for
	// a human to judge; the two named forms have no legitimate engine use.
	for _, f := range enginesSourceFiles(t) {
		data, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		if strings.Contains(src, "os.Exit") {
			t.Errorf("%s calls os.Exit -- a library must never exit its host", f)
		}
		if strings.Contains(src, "os.Stderr") || strings.Contains(src, "os.Stdout") {
			t.Errorf("%s touches os.Stderr/os.Stdout -- record conditions on Block.Data for the host to surface", f)
		}
	}
}
