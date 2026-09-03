package plugin_save

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

type Save struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (s *Save) Init() error {
	s.name = "save2file"
	s.version = "0.0.1"
	s.description = "Save text to a new text file in Downloads and reveal it"
	s.command = "save2file"
	s.args = map[string]string{
		"-dir <path>":  "Output directory (default: system Downloads)",
		"-name <name>": "Filename (default: <unix_seconds>.txt)",
		"-out <path>":  "Full output file path (overrides -dir and -name)",
		"-clip":        "Read input from the clipboard",
		"-pipe":        "Read input from pipe/stdin (auto-detected)",
		"-no-reveal":   "Skip opening the file in the file manager",
		"-h":           "Show help",
	}
	s.author = "vst"
	return nil
}

func (s *Save) GetName() string            { return s.name }
func (s *Save) GetVersion() string         { return s.version }
func (s *Save) GetDescription() string     { return s.description }
func (s *Save) GetCommand() string         { return s.command }
func (s *Save) GetArgs() map[string]string { return s.args }
func (s *Save) GetAuthor() string          { return s.author }
func (s *Save) GetAliases() []string       { return []string{"s2f"} }
func (s *Save) Stop() error                { return nil }

type saveOptions struct {
	input      string
	hasPipe    bool
	fromClip   bool
	dir        string
	name       string
	out        string
	noReveal   bool
	positional []string
}

func parseSaveArgs(args []string) (saveOptions, bool, error) {
	var opts saveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-dir", "--dir":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("-dir requires a path")
			}
			opts.dir = args[i+1]
			i++
		case "-name", "--name":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("-name requires a filename")
			}
			opts.name = args[i+1]
			i++
		case "-out", "--out":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("-out requires a path")
			}
			opts.out = args[i+1]
			i++
		case "-clip":
			opts.fromClip = true
		case "-no-reveal":
			opts.noReveal = true
		case "-pipe":
			if i+1 < len(args) {
				opts.input = args[i+1]
				opts.hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
			return opts, true, nil
		default:
			if !strings.HasPrefix(args[i], "-") {
				opts.positional = append(opts.positional, args[i])
			} else {
				return opts, false, fmt.Errorf("unknown option %q; run `v save2file -h` for help", args[i])
			}
		}
	}
	return opts, false, nil
}

func readClipboardText() (string, bool) {
	clip, err := clipboard.ReadAll()
	if err != nil {
		return "", false
	}
	return clip, clip != ""
}

func resolveSaveInput(opts saveOptions) (string, bool) {
	if opts.hasPipe {
		return opts.input, opts.input != ""
	}
	if opts.fromClip {
		return readClipboardText()
	}
	if len(opts.positional) > 0 {
		return strings.Join(opts.positional, " "), true
	}
	return readClipboardText()
}

func buildSavePath(opts saveOptions) (string, error) {
	if opts.out != "" {
		return expandHome(opts.out), nil
	}
	dir := opts.dir
	if dir == "" {
		dir = defaultDownloadsDir()
	} else {
		dir = expandHome(dir)
	}
	name := opts.name
	if name == "" {
		name = fmt.Sprintf("%d.txt", time.Now().Unix())
	}
	if name == "." || name == ".." || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid filename %q; use -out to set a full path", name)
	}
	return filepath.Join(dir, name), nil
}

func (s *Save) Run(args []string) error {
	opts, help, err := parseSaveArgs(args)
	if err != nil {
		return err
	}
	if help {
		s.printHelp()
		return nil
	}

	text, ok := resolveSaveInput(opts)
	if !ok {
		fmt.Println("No text to save. Pass text as an argument, pipe it in, or put text in the clipboard first.")
		return nil
	}

	path, err := buildSavePath(opts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Saved %d bytes to %s\n", len(text), path)
	if opts.noReveal {
		return nil
	}
	if err := revealFile(path); err != nil {
		fmt.Printf("⚠️  File saved, but could not open the folder: %v\n", err)
	}
	return nil
}

func (s *Save) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>save2file - Save Text to File v%s</> <gray>(alias: s2f)</>\n\n", s.version)
	color.Println("Write text to a new file with automatic file reveal in the file manager.")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v save2file                              Save clipboard to ~/Downloads/<timestamp>.txt")
	color.Println("  echo \"hello\" | v save2file                Save piped text")
	color.Println("  v save2file \"hello world\"                Save text argument")
	color.Println("  v save2file <green>-name</> notes.txt <green>-dir</> ~/notes  Custom filename and directory")
	color.Println("  v save2file <green>-out</> /tmp/note.txt          Full custom path")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-dir</> <path>      Output directory (default: system Downloads)")
	color.Println("  <green>-name</> <name>     Filename (default: <unix_seconds>.txt)")
	color.Println("  <green>-out</> <path>      Full output file path (overrides -dir and -name)")
	color.Println("  <green>-clip</>            Read input from clipboard (default)")
	color.Println("  <green>-pipe</>            Read from pipe/stdin (auto-detected)")
	color.Println("  <green>-no-reveal</>       Skip opening the file in file manager")
	color.Println("  <green>-h</>               Show this help")
	color.Println()
	color.Println("<fg=magenta;op=bold>Examples:</>")
	color.Println("  <gray># Quick save clipboard to Downloads</>")
	color.Println("  v s2f")
	color.Println()
	color.Println("  <gray># Save with custom name</>")
	color.Println("  v save2file <green>-name</> \"meeting-notes.txt\"")
	color.Println()
	color.Println("  <gray># Pipe output from another command</>")
	color.Println("  cat data.txt | grep \"error\" | v s2f <green>-name</> errors.txt")
	color.Println()
	color.Println("<gray>Input priority: pipe > argument > clipboard</>")
	color.Println("<gray>Reveal: 'open -R' on macOS, Explorer select on Windows, xdg-open on Linux</>")
	color.Println("<gray>--------------------------------------------------</>")
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}
