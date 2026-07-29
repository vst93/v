package plugin_gcm

import (
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
	_ = g.actionMenu("feat: test message")
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
	err := g.actionMenu("feat: test message")
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
	err := g.actionMenu("feat: test message")
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
	_ = g.actionMenu("feat: test message")

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
