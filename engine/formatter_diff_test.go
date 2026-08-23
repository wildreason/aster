package engine

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseHunks(t *testing.T) {
	diffContent := `--- a/file.go
+++ b/file.go
@@ -3,6 +3,8 @@

 import "fmt"

+const TaxRate = 0.08
+
 func calculateTotal(items []int) int {
     sum := 0
     for _, item := range items {
@@ -11,9 +13,15 @@
     return sum
 }

+func calculateWithTax(subtotal int) float64 {
+    return float64(subtotal) * (1 + TaxRate)
+}
+
 func main() {
     numbers := []int{1, 2, 3, 4, 5}
-    result := calculateTotal(numbers)
-    fmt.Println("Total:", result)
+    subtotal := calculateTotal(numbers)
+    total := calculateWithTax(subtotal)
+    fmt.Printf("Subtotal: %d\n", subtotal)
+    fmt.Printf("Total with tax: %.2f\n", total)
 }
`

	hunks := ParseHunks(diffContent)

	if len(hunks) != 2 {
		t.Errorf("Expected 2 hunks, got %d", len(hunks))
	}

	// First hunk should have additions
	firstHunk := hunks[0]
	addedCount := 0
	for _, line := range firstHunk.Lines {
		if line.Type == DiffAdded {
			addedCount++
		}
	}
	if addedCount != 2 {
		t.Errorf("First hunk: expected 2 additions, got %d", addedCount)
	}

	// Second hunk should have both additions and deletions
	secondHunk := hunks[1]
	addedCount = 0
	removedCount := 0
	for _, line := range secondHunk.Lines {
		if line.Type == DiffAdded {
			addedCount++
		}
		if line.Type == DiffRemoved {
			removedCount++
		}
	}
	if removedCount != 2 {
		t.Errorf("Second hunk: expected 2 deletions, got %d", removedCount)
	}
}

func TestDetectFunctions(t *testing.T) {
	formatter := NewDiffFormatter(80)

	// Test Go function detection
	hunk := DiffHunk{
		Lines: []DiffLine{
			{Type: DiffAdded, Content: "func calculateWithTax(subtotal int) float64 {"},
			{Type: DiffAdded, Content: "    return float64(subtotal) * (1 + TaxRate)"},
			{Type: DiffAdded, Content: "}"},
		},
	}

	result := formatter.detectFunctions(hunk)
	if !strings.Contains(result, "calculateWithTax()") {
		t.Errorf("Expected to detect calculateWithTax(), got: %s", result)
	}
}

func TestDiffContentDetection(t *testing.T) {
	// Must include file headers for strict diff detection
	diffContent := `--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 line1
+added
 line2
 line3`

	ct := DetectBlockContentType(diffContent)
	if ct != BlockContentDiff {
		t.Errorf("Expected BlockContentDiff, got %s", ct.String())
	}

	plainContent := "This is just plain text\nwith multiple lines"
	ct = DetectBlockContentType(plainContent)
	if ct != BlockContentPlain {
		t.Errorf("Expected BlockContentPlain, got %s", ct.String())
	}
}

func TestFormatLine(t *testing.T) {
	formatter := NewDiffFormatter(80)

	// Test added line has background color (dark green: #2d5a2d)
	addedLine := DiffLine{Type: DiffAdded, Content: "added line"}
	result := formatter.formatLine(addedLine, 40)

	// Should contain ANSI background code for dark green
	if !strings.Contains(result, "\033[48;2;45;90;45m") {
		t.Error("Added line should have dark green background")
	}

	// Test removed line has background color (dark magenta: #5a2d5a)
	removedLine := DiffLine{Type: DiffRemoved, Content: "removed line"}
	result = formatter.formatLine(removedLine, 40)

	// Should contain ANSI background code for dark magenta
	if !strings.Contains(result, "\033[48;2;90;45;90m") {
		t.Error("Removed line should have dark magenta background")
	}
}

// Run this to see visual output
func ExampleDiffFormatter() {
	diffContent := `--- a/file.go
+++ b/file.go
@@ -3,6 +3,8 @@

 import "fmt"

+const TaxRate = 0.08
+
 func calculateTotal(items []int) int {
`

	formatter := NewDiffFormatter(80)
	output := formatter.Format(diffContent, "file.go")
	fmt.Println(output)
}

func TestDiffParserDetect(t *testing.T) {
	parser := &DiffParser{}

	// Should detect .diff files
	if !parser.Detect("changes.diff") {
		t.Error("Should detect .diff files")
	}

	// Should detect .patch files
	if !parser.Detect("fix.patch") {
		t.Error("Should detect .patch files")
	}

	// Should not detect other files
	if parser.Detect("file.go") {
		t.Error("Should not detect .go files")
	}
}

func TestDiffParserParse(t *testing.T) {
	diffContent := `--- a/file.go
+++ b/file.go
@@ -3,6 +3,8 @@

 import "fmt"

+const TaxRate = 0.08
+
 func calculateTotal(items []int) int {
@@ -11,9 +13,15 @@
     return sum
 }

+func calculateWithTax(subtotal int) float64 {
+    return float64(subtotal) * (1 + TaxRate)
+}
`

	parser := &DiffParser{}
	blocks := parser.Parse(diffContent)

	if len(blocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(blocks))
	}

	if len(blocks) > 0 {
		block := blocks[0]
		// Should have 2 pages (2 hunks)
		if block.TotalPages != 2 {
			t.Errorf("Expected 2 pages (hunks), got %d", block.TotalPages)
		}
		// ContentType should be set to diff
		if block.ContentType != BlockContentDiff {
			t.Errorf("Expected ContentType=BlockContentDiff, got %s", block.ContentType.String())
		}
		// PageTypes should be set for each page
		if len(block.PageTypes) != 2 {
			t.Errorf("Expected 2 PageTypes, got %d", len(block.PageTypes))
		}
		for i, pt := range block.PageTypes {
			if pt != BlockContentDiff {
				t.Errorf("PageTypes[%d] should be BlockContentDiff, got %s", i, pt.String())
			}
		}
	}
}
