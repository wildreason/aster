package engine

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_Basic(t *testing.T) {
	content := "---\ntitle: Test Doc\ncreated: 2026-02-23\ntags: [demo, test]\n---\n\n# Hello\n\nWorld"
	fm, body := ParseFrontmatter(content)

	if fm.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got %q", fm.Title)
	}
	if fm.Created != "2026-02-23" {
		t.Errorf("expected created '2026-02-23', got %q", fm.Created)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "demo" || fm.Tags[1] != "test" {
		t.Errorf("expected tags [demo, test], got %v", fm.Tags)
	}
	if !strings.HasPrefix(body, "# Hello") {
		t.Errorf("expected body to start with '# Hello', got %q", body[:min(len(body), 20)])
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just a heading\n\nSome text."
	fm, body := ParseFrontmatter(content)

	if fm.Title != "" {
		t.Errorf("expected empty title, got %q", fm.Title)
	}
	if body != content {
		t.Errorf("expected body to equal original content")
	}
}

func TestParseFrontmatter_EmptyTags(t *testing.T) {
	content := "---\ntitle: No Tags\ntags: []\n---\n\nBody"
	fm, body := ParseFrontmatter(content)

	if fm.Title != "No Tags" {
		t.Errorf("expected title 'No Tags', got %q", fm.Title)
	}
	if len(fm.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", fm.Tags)
	}
	if body != "Body" {
		t.Errorf("expected body 'Body', got %q", body)
	}
}

func TestParseFrontmatter_RawFields(t *testing.T) {
	content := "---\ntitle: Doc\nauthor: Alice\nstatus: draft\n---\n\nContent"
	fm, _ := ParseFrontmatter(content)

	if fm.Raw["author"] != "Alice" {
		t.Errorf("expected raw author 'Alice', got %q", fm.Raw["author"])
	}
	if fm.Raw["status"] != "draft" {
		t.Errorf("expected raw status 'draft', got %q", fm.Raw["status"])
	}
}

func TestParseFrontmatter_UnclosedDelimiter(t *testing.T) {
	content := "---\ntitle: Broken\nNo closing delimiter"
	fm, body := ParseFrontmatter(content)

	if fm.Title != "" {
		t.Errorf("expected empty title for unclosed frontmatter, got %q", fm.Title)
	}
	if body != content {
		t.Errorf("expected body to equal original content")
	}
}

func TestParseBracketList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"[a, b, c]", []string{"a", "b", "c"}},
		{"[]", nil},
		{"[single]", []string{"single"}},
		{"plain", []string{"plain"}},
		{"", nil},
	}

	for _, tc := range tests {
		result := parseBracketList(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("parseBracketList(%q): expected %v, got %v", tc.input, tc.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("parseBracketList(%q)[%d]: expected %q, got %q", tc.input, i, tc.expected[i], result[i])
			}
		}
	}
}
