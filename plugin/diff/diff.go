package plugin_diff

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

type Diff struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (d *Diff) Init() error {
	d.name = "diff"
	d.version = "1.0.0"
	d.description = "Side-by-side text diff viewer with search and inline word-level highlighting"
	d.command = "diff"
	d.args = map[string]string{
		"-left":   "Left file path",
		"-right":  "Right file path",
		"-clip":   "Read clipboard as a source (use with -left or -right)",
		"-pipe":   "Read from pipe/stdin (auto-detected, used as left side)",
		"-inline": "Inline (unified) diff output instead of TUI",
		"-raw":    "Plain text output (no colors, use with -inline)",
		"-h":      "Show help",
	}
	d.author = "vst"
	return nil
}

func (d *Diff) GetName() string            { return d.name }
func (d *Diff) GetVersion() string         { return d.version }
func (d *Diff) GetDescription() string     { return d.description }
func (d *Diff) GetCommand() string         { return d.command }
func (d *Diff) GetArgs() map[string]string { return d.args }
func (d *Diff) GetAuthor() string          { return d.author }
func (d *Diff) Stop() error                { return nil }

func (d *Diff) Run(args []string) error {
	var (
		leftPath  string
		rightPath string
		inline    bool
		raw       bool
		pipeData  string
		hasPipe   bool
		useClip   bool
	)

	// Parse args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-left":
			if i+1 < len(args) {
				leftPath = args[i+1]
				i++
			}
		case "-right":
			if i+1 < len(args) {
				rightPath = args[i+1]
				i++
			}
		case "-inline":
			inline = true
		case "-raw":
			raw = true
		case "-clip":
			useClip = true
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
			d.printHelp()
			return nil
		}
	}

	// No inputs: open paste mode so the user can paste left/right text
	// directly into the TUI. The clipboard is never read automatically.
	if !hasPipe && leftPath == "" && rightPath == "" && !useClip {
		return NewPasteViewer().Run()
	}

	// Gather left and right text
	var leftText, rightText string
	var leftName, rightName string

	// Determine left source
	switch {
	case hasPipe:
		leftText = pipeData
		leftName = "pipe"
	case leftPath != "":
		data, err := os.ReadFile(expandHome(leftPath))
		if err != nil {
			return fmt.Errorf("failed to read left file: %w", err)
		}
		leftText = string(data)
		leftName = leftPath
	case useClip:
		content, err := clipboard.ReadAll()
		if err != nil {
			return fmt.Errorf("failed to read clipboard: %w", err)
		}
		leftText = content
		leftName = "clipboard"
	default:
		return fmt.Errorf("no left input specified. Use -left <file>, -clip, or pipe input")
	}

	// Determine right source
	switch {
	case rightPath != "":
		data, err := os.ReadFile(expandHome(rightPath))
		if err != nil {
			return fmt.Errorf("failed to read right file: %w", err)
		}
		rightText = string(data)
		rightName = rightPath
	case useClip && leftPath != "":
		// -clip + -left: compare clipboard (right) against file (left)
		content, err := clipboard.ReadAll()
		if err != nil {
			return fmt.Errorf("failed to read clipboard: %w", err)
		}
		rightText = content
		rightName = "clipboard"
	default:
		return fmt.Errorf("no right input specified. Use -right <file>")
	}

	leftText = strings.TrimSpace(leftText)
	rightText = strings.TrimSpace(rightText)

	if leftText == "" && rightText == "" {
		return fmt.Errorf("both sides are empty")
	}

	// Compute diff
	lines := DiffLines(leftText, rightText)

	if inline {
		return d.printInline(lines, leftName, rightName, raw)
	}

	// Launch interactive TUI
	dv := NewDiffViewer(lines, leftName, rightName)
	return dv.Run()
}

// printInline outputs a unified diff to stdout.
func (d *Diff) printInline(lines []DiffLine, leftName, rightName string, raw bool) error {
	if raw {
		fmt.Printf("--- %s\n+++ %s\n", leftName, rightName)
		for _, line := range lines {
			switch line.Op {
			case OpEqual:
				fmt.Printf("  %s\n", line.Left)
			case opChange:
				fmt.Printf("- %s\n+ %s\n", line.Left, line.Right)
			case OpDel:
				fmt.Printf("- %s\n", line.Left)
			case OpAdd:
				fmt.Printf("+ %s\n", line.Right)
			}
		}
		return nil
	}

	// Colored inline output
	for _, line := range lines {
		switch line.Op {
		case OpEqual:
			fmt.Printf("\033[90m  %s\033[0m\n", line.Left)
		case opChange:
			fmt.Printf("\033[31m- %s\033[0m\n", line.Left)
			fmt.Printf("\033[32m+ %s\033[0m\n", line.Right)
		case OpDel:
			fmt.Printf("\033[31m- %s\033[0m\n", line.Left)
		case OpAdd:
			fmt.Printf("\033[32m+ %s\033[0m\n", line.Right)
		}
	}
	return nil
}

func (d *Diff) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>diff - Side-by-side Text Diff Viewer v%s</>\n\n", d.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v diff <green>-left</> <f1> <green>-right</> <f2>    Compare two files (interactive TUI)")
	color.Println("  v diff <green>-left</> <file> <green>-clip</>        Compare file vs clipboard")
	color.Println("  echo 'text' | v diff <green>-right</> <f>   Compare pipe vs file")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-left</> <path>   Left file path")
	color.Println("  <green>-right</> <path>  Right file path")
	color.Println("  <green>-inline</>        Unified diff to stdout (no TUI)")
	color.Println("  <green>-raw</>           Plain text output (with -inline)")
	color.Println()
	color.Println("<gray>I/O: -pipe (auto, as left side) · -clip · -h</>")
	color.Println()
	color.Println("<gray>Viewer: ↑↓/jk nav · n/N next-prev hunk · / search · c changed · a all · e edit · q quit</>")
	color.Println("<gray>Paste:  Tab switch · Ctrl-A/C/X/V · Ctrl-D diff · Esc quit</>")
	color.Println("<gray>Colors: red=deleted · green=added · orange=changed</>")
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
