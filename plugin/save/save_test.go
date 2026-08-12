package plugin_save

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestParseSaveArgsDefaults(t *testing.T) {
	opts, help, err := parseSaveArgs(nil)
	if err != nil {
		t.Fatalf("parseSaveArgs returned an error: %v", err)
	}
	if help {
		t.Fatal("parseSaveArgs unexpectedly requested help")
	}
	if opts.dir != "" || opts.name != "" || opts.out != "" || opts.fromClip || opts.noReveal || len(opts.positional) != 0 {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestParseSaveArgsValues(t *testing.T) {
	opts, _, err := parseSaveArgs([]string{"-dir", "~/x", "-name", "note.txt", "-clip", "-no-reveal", "hello", "world"})
	if err != nil {
		t.Fatalf("parseSaveArgs returned an error: %v", err)
	}
	if opts.dir != "~/x" {
		t.Errorf("dir = %q, want %q", opts.dir, "~/x")
	}
	if opts.name != "note.txt" {
		t.Errorf("name = %q, want %q", opts.name, "note.txt")
	}
	if !opts.fromClip {
		t.Error("fromClip = false, want true")
	}
	if !opts.noReveal {
		t.Error("noReveal = false, want true")
	}
	if len(opts.positional) != 2 || opts.positional[0] != "hello" || opts.positional[1] != "world" {
		t.Errorf("positional = %v, want [hello world]", opts.positional)
	}
}

func TestParseSaveArgsPipe(t *testing.T) {
	opts, _, err := parseSaveArgs([]string{"-pipe", "hi"})
	if err != nil {
		t.Fatalf("parseSaveArgs returned an error: %v", err)
	}
	if !opts.hasPipe || opts.input != "hi" {
		t.Errorf("pipe input = (%v, %q), want (true, %q)", opts.hasPipe, opts.input, "hi")
	}
}

func TestParseSaveArgsHelp(t *testing.T) {
	if _, help, err := parseSaveArgs([]string{"--help"}); err != nil {
		t.Fatalf("parseSaveArgs returned an error: %v", err)
	} else if !help {
		t.Fatal("parseSaveArgs did not request help")
	}
}

func TestParseSaveArgsMissingValue(t *testing.T) {
	if _, _, err := parseSaveArgs([]string{"-dir"}); err == nil {
		t.Fatal("parseSaveArgs accepted -dir without a value")
	}
}

func TestParseSaveArgsUnknown(t *testing.T) {
	if _, _, err := parseSaveArgs([]string{"-bogus"}); err == nil {
		t.Fatal("parseSaveArgs accepted an unknown option")
	}
}

func TestResolveSaveInputPipe(t *testing.T) {
	text, ok := resolveSaveInput(saveOptions{input: "piped", hasPipe: true})
	if !ok || text != "piped" {
		t.Errorf("pipe input = (%q, %v), want (%q, true)", text, ok, "piped")
	}
}

func TestResolveSaveInputPositional(t *testing.T) {
	text, ok := resolveSaveInput(saveOptions{positional: []string{"a", "b"}})
	if !ok || text != "a b" {
		t.Errorf("positional input = (%q, %v), want (%q, true)", text, ok, "a b")
	}
}

func TestResolveSaveInputPipeWinsOverClip(t *testing.T) {
	text, ok := resolveSaveInput(saveOptions{input: "piped", hasPipe: true, fromClip: true})
	if !ok || text != "piped" {
		t.Errorf("input = (%q, %v), want (%q, true)", text, ok, "piped")
	}
}

func TestBuildSavePathCustomOut(t *testing.T) {
	path, err := buildSavePath(saveOptions{out: "/tmp/a.txt"})
	if err != nil {
		t.Fatalf("buildSavePath returned an error: %v", err)
	}
	if path != "/tmp/a.txt" {
		t.Errorf("path = %q, want %q", path, "/tmp/a.txt")
	}
}

func TestBuildSavePathCustomDirName(t *testing.T) {
	path, err := buildSavePath(saveOptions{dir: "/tmp", name: "note.md"})
	if err != nil {
		t.Fatalf("buildSavePath returned an error: %v", err)
	}
	if path != filepath.Join("/tmp", "note.md") {
		t.Errorf("path = %q, want %q", path, filepath.Join("/tmp", "note.md"))
	}
}

func TestBuildSavePathDefaultTimestamp(t *testing.T) {
	path, err := buildSavePath(saveOptions{})
	if err != nil {
		t.Fatalf("buildSavePath returned an error: %v", err)
	}
	re := regexp.MustCompile(`^\d{10}\.txt$`)
	if !re.MatchString(filepath.Base(path)) {
		t.Errorf("default filename = %q, want a 10-digit timestamp with .txt", filepath.Base(path))
	}
	if filepath.Dir(path) == "" {
		t.Error("default directory is empty")
	}
}

func TestBuildSavePathRejectsAbsoluteName(t *testing.T) {
	if _, err := buildSavePath(saveOptions{name: "/abs"}); err == nil {
		t.Fatal("buildSavePath accepted an absolute filename")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	if got := expandHome("~/x"); got != filepath.Join(home, "x") {
		t.Errorf("expandHome(~/x) = %q, want %q", got, filepath.Join(home, "x"))
	}
	if got := expandHome("/x"); got != "/x" {
		t.Errorf("expandHome(/x) = %q, want %q", got, "/x")
	}
}
