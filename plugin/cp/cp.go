package plugin_cp

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

type Cp struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (c *Cp) Init() error {
	c.name = "cp"
	c.version = "0.0.1"
	c.description = "Copy text to clipboard (designed for pipe mode)"
	c.command = "cp"
	c.args = map[string]string{
		"-trim":  "Trim leading/trailing whitespace (spaces, tabs, newlines)",
		"-triml": "Trim leading whitespace only",
		"-trimr": "Trim trailing whitespace only",
		"-pipe":  "Read from pipe/stdin (auto-detected)",
		"-h":     "Show help",
	}
	c.author = "vst"
	return nil
}

func (c *Cp) GetName() string            { return c.name }
func (c *Cp) GetVersion() string         { return c.version }
func (c *Cp) GetDescription() string     { return c.description }
func (c *Cp) GetCommand() string         { return c.command }
func (c *Cp) GetArgs() map[string]string { return c.args }
func (c *Cp) GetAuthor() string          { return c.author }
func (c *Cp) GetAliases() []string       { return nil }
func (c *Cp) Stop() error                { return nil }

func (c *Cp) Run(args []string) error {
	var (
		pipeData  string
		hasPipe   bool
		trim      bool
		trimLeft  bool
		trimRight bool
		input     string
		hasInput  bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-trim":
			trim = true
		case "-triml":
			trimLeft = true
		case "-trimr":
			trimRight = true
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
			c.printHelp()
			return nil
		default:
			if !strings.HasPrefix(arg, "-") && !hasInput {
				input = arg
				hasInput = true
			}
		}
	}

	// Get input: pipe > argument > clipboard
	switch {
	case hasPipe:
		input = pipeData
	case !hasInput:
		clip, err := clipboard.ReadAll()
		if err != nil || clip == "" {
			return fmt.Errorf("no input: pipe text in, pass as argument, or have text in clipboard")
		}
		input = clip
	}

	// Apply trimming
	if trim {
		input = strings.TrimSpace(input)
	} else {
		if trimLeft {
			input = strings.TrimLeft(input, " \t\n\r")
		}
		if trimRight {
			input = strings.TrimRight(input, " \t\n\r")
		}
	}

	if input == "" {
		return fmt.Errorf("input is empty (after trimming if applied)")
	}

	if err := clipboard.WriteAll(input); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	fmt.Println("✅ Copied to clipboard")
	return nil
}

func (c *Cp) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Println("<fg=cyan;op=bold>📦 cp - Copy to Clipboard</>")
	color.Println()
	color.Println("Copy text to clipboard. Designed for pipe mode to chain with other commands.")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println(`  echo "text" | v cp              Copy piped stdin to clipboard`)
	color.Println(`  v cp "some text"                Copy argument to clipboard`)
	color.Println(`  echo "  hi  " | v cp <green>-trim</>      Trim whitespace then copy`)
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-trim</>   Trim leading & trailing whitespace (spaces, tabs, newlines)")
	color.Println("  <green>-triml</>  Trim leading whitespace only")
	color.Println("  <green>-trimr</>  Trim trailing whitespace only")
	color.Println("  <green>-h</>      Show this help")
	color.Println()
	color.Println("<gray>Input priority: pipe > argument > clipboard</>")
	color.Println("<gray>--------------------------------------------------</>")
}
