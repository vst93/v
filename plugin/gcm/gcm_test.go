package plugin_gcm

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// TestActionMenuCopy verifies that choosing "1" (or empty = default)
// routes to the copy path. Clipboard may fail in headless env, so
// we only check that it doesn't panic and produces some output.
func TestActionMenuCopy(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	input := "\n" // empty = default = copy
	r, w, _ := os.Pipe()
	w.Write([]byte(input))
	w.Close()
	os.Stdin = r

	g := &Gcm{}
	_ = g.actionMenu("feat: test message", "", "staged", false)
	// No error check: clipboard may fail in headless CI.
	// The test verifies the code path doesn't panic.
}

// TestActionMenuDoNothing verifies option 4 does nothing.
func TestActionMenuDoNothing(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	w.Write([]byte("4\n"))
	w.Close()
	os.Stdin = r

	g := &Gcm{}
	err := g.actionMenu("feat: test message", "", "staged", false)
	if err != nil {
		t.Fatalf("actionMenu returned error: %v", err)
	}
}

// TestActionMenuUnknown verifies unknown input doesn't crash.
func TestActionMenuUnknown(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	w.Write([]byte("xyz\n"))
	w.Close()
	os.Stdin = r

	g := &Gcm{}
	err := g.actionMenu("feat: test message", "", "staged", false)
	if err != nil {
		t.Fatalf("actionMenu returned error: %v", err)
	}
}

// TestActionMenuOutput verifies the menu text contains expected options.
func TestActionMenuOutput(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout via pipe
	rIn, wIn, _ := os.Pipe()
	wIn.Write([]byte("4\n"))
	wIn.Close()
	os.Stdin = rIn

	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	g := &Gcm{}
	_ = g.actionMenu("feat: test message", "", "staged", false)

	wOut.Close()
	out, _ := io.ReadAll(rOut)

	output := string(out)
	checks := []string{
		"What would you like to do?",
		"Copy to clipboard",
		"Commit with this message",
		"Commit and push",
		"Do nothing",
		"Choice [1-4]",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q, got: %s", check, output)
		}
	}
}

// TestCondenseDiffSmall returns the diff unchanged when under the limit.
func TestCondenseDiffSmall(t *testing.T) {
	diff := "diff --git a/foo b/foo\nindex abc..def 100644\n--- a/foo\n+++ b/foo\n@@ -1,3 +1,3 @@\n old\n-old line\n+new line\n"
	got := condenseDiff(diff, 10000)
	if got != diff {
		t.Errorf("condenseDiff changed a small diff")
	}
}

// TestCondenseDiffLarge preserves all file headers when the diff is large.
func TestCondenseDiffLarge(t *testing.T) {
	var diff strings.Builder
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&diff, "diff --git a/file%d.go b/file%d.go\n", i, i)
		diff.WriteString("index abc..def 100644\n")
		diff.WriteString("--- a/file.go\n+++ b/file.go\n")
		fmt.Fprintf(&diff, "@@ -1,100 +1,100 @@\n")
		for j := 0; j < 100; j++ {
			fmt.Fprintf(&diff, "-old line %d\n", j)
			fmt.Fprintf(&diff, "+new line %d\n", j)
		}
	}

	got := condenseDiff(diff.String(), 2000)
	// Every file header must survive.
	for i := 0; i < 5; i++ {
		want := fmt.Sprintf("diff --git a/file%d.go", i)
		if !strings.Contains(got, want) {
			t.Errorf("condensed diff lost file header %q", want)
		}
	}
	// Must be under budget (or very close).
	if len(got) > 3000 {
		t.Errorf("condensed diff too large: %d chars", len(got))
	}
	// Should contain trim markers.
	if !strings.Contains(got, "more changes trimmed") {
		t.Errorf("condensed diff should contain trim markers")
	}
}

// TestCondenseDiffManyFiles keeps headers only when budget is tiny.
func TestCondenseDiffManyFiles(t *testing.T) {
	var diff strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&diff, "diff --git a/file%d.go b/file%d.go\n", i, i)
		diff.WriteString("@@ -1,5 +1,5 @@\n-old\n+new\n")
	}

	got := condenseDiff(diff.String(), 500)
	// At least some file headers survive.
	if !strings.Contains(got, "diff --git a/file0.go") {
		t.Errorf("condensed diff lost file0 header")
	}
}

// TestDiffFileSummary builds a stat from pipe diff text.
func TestDiffFileSummary(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,3 @@
-old
+new
 context
diff --git a/bar.go b/bar.go
index abc..def 100644
--- a/bar.go
+++ b/bar.go
@@ -1,2 +1,4 @@
+added1
+added2
 context
-deleted
`
	stat := diffFileSummary(diff)
	if !strings.Contains(stat, "foo.go") {
		t.Errorf("stat missing foo.go")
	}
	if !strings.Contains(stat, "bar.go") {
		t.Errorf("stat missing bar.go")
	}
	if !strings.Contains(stat, "2 files changed") {
		t.Errorf("stat missing file count")
	}
	// foo.go: 1 added, 1 deleted; bar.go: 2 added, 1 deleted
	if !strings.Contains(stat, "3 insertions") {
		t.Errorf("stat missing total insertions")
	}
	if !strings.Contains(stat, "2 deletions") {
		t.Errorf("stat missing total deletions")
	}
}

// TestExtractFileName pulls file paths from diff headers.
func TestExtractFileName(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"diff --git a/src/foo.go b/src/foo.go\n", "src/foo.go"},
		{"diff --git a/bar.txt b/bar.txt\n", "bar.txt"},
		{"no diff header here\n", ""},
	}
	for _, c := range cases {
		got := extractFileName(c.input)
		if got != c.want {
			t.Errorf("extractFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestCountChanges counts added/deleted lines.
func TestCountChanges(t *testing.T) {
	diff := `diff --git a/foo b/foo
--- a/foo
+++ b/foo
@@ -1,3 +1,3 @@
-old
+new
 context
-deleted
+added
`
	add, del := countChanges(diff)
	if add != 2 || del != 2 {
		t.Errorf("countChanges = (%d, %d), want (2, 2)", add, del)
	}
}
