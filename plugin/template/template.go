// Package plugin_template is a copy-paste scaffold for new plugins. It is
// deliberately NOT registered in service/plugin.go.
//
// Flag convention (see AGENTS.md). These names mean the same thing in every
// plugin, so never reuse them for something else:
//
//	-pipe          pipe/stdin input (main.go appends `-pipe <data>` automatically)
//	-file <path>   read input from a file
//	-clip          read the clipboard as input
//	-url <url>     read input from a URL
//	-out <path>    write the result to a file
//	-copy          write the result to the clipboard
//	-tui           interactive TUI mode
//	-raw           plain text output, no colors
//	-h             help (accept -h, -help and --help together)
//
// Input priority: -pipe > -file > -url > positional argument > clipboard.
// Plugin-specific flags use multi-letter words (-sort, -inline, -trim, ...).
package plugin_template

import (
	"fmt"
	"strings"
)

type Template struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (t *Template) Init() error {
	t.name = "template"
	t.version = "0.0.1"
	t.description = "This is a template plugin."
	t.command = "template"
	t.args = map[string]string{
		"-pipe": "Read from pipe/stdin (auto-detected)",
		"-copy": "Copy the result to clipboard",
		"-h":    "Show help",
	}
	t.author = ""
	return nil
}

func (t *Template) GetName() string {
	return t.name
}
func (t *Template) GetVersion() string {
	return t.version
}
func (t *Template) GetDescription() string {
	return t.description
}
func (t *Template) GetCommand() string {
	return t.command
}
func (t *Template) GetArgs() map[string]string {
	return t.args
}
func (t *Template) GetAuthor() string {
	return t.author
}

func (t *Template) Run(args []string) error {
	var (
		pipeData string
		hasPipe  bool
		toClip   bool
		input    string
		hasInput bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-copy":
			toClip = true
		case "-h", "-help", "--help":
			t.printHelp()
			return nil
		default:
			// Positional argument = literal input text.
			if !strings.HasPrefix(args[i], "-") && !hasInput {
				input = args[i]
				hasInput = true
			}
		}
	}

	if hasPipe {
		input = pipeData
	}
	_ = toClip

	// TODO: implement your plugin logic here
	_ = input
	return nil
}

func (t *Template) printHelp() {
	fmt.Printf("template - %s v%s\n\n", t.description, t.version)
	fmt.Println("Usage:")
	fmt.Println("  v template [input] [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -pipe   Read from stdin/pipe (auto-detected)")
	fmt.Println("  -copy   Copy the result to clipboard")
	fmt.Println("  -h      Show this help")
}

func (t *Template) Stop() error {
	return nil
}
