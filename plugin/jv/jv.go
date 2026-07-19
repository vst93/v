package plugin_jv

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
)

type Jv struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (j *Jv) Init() error {
	j.name = "jv"
	j.version = "1.0.0"
	j.description = "JSON Viewer & Formatter - format, compress, escape, and interactively browse JSON"
	j.command = "jv"
	j.args = map[string]string{
		"-f":     "Format (pretty-print) JSON from clipboard/file/pipe/stdin",
		"-c":     "Compress (minify) JSON to a single line",
		"-e":     "Escape non-ASCII to \\uXXXX sequences",
		"-u":     "Unescape \\uXXXX sequences to UTF-8 text",
		"-i":     "Interactive tree viewer (default when no flag given)",
		"-sort":  "Sort object keys alphabetically",
		"-file":  "Read from file path instead of clipboard",
		"-raw":   "Disable colored output (plain text)",
		"-pipe":  "Read from pipe/stdin (auto-detected)",
	}
	j.author = "vst"
	return nil
}

func (j *Jv) GetName() string        { return j.name }
func (j *Jv) GetVersion() string     { return j.version }
func (j *Jv) GetDescription() string { return j.description }
func (j *Jv) GetCommand() string     { return j.command }
func (j *Jv) GetArgs() map[string]string { return j.args }
func (j *Jv) GetAuthor() string      { return j.author }

func (j *Jv) Stop() error { return nil }

func (j *Jv) Run(args []string) error {
	// Parse flags
	var (
		mode      string // "format", "compress", "escape", "unescape", "interactive"
		filePath  string
		raw       bool
		sortKeys  bool
		pipeData  string
		hasPipe   bool
	)

	mode = "interactive" // default

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-f", "--format":
			mode = "format"
		case "-c", "--compress":
			mode = "compress"
		case "-e", "--escape":
			mode = "escape"
		case "-u", "--unescape":
			mode = "unescape"
		case "-i", "--interactive":
			mode = "interactive"
		case "-sort", "--sort":
			sortKeys = true
		case "-raw", "--raw":
			raw = true
		case "-file", "--file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		case "-pipe", "--pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "--help":
			j.printHelp()
			return nil
		}
	}

	// Get input data
	var inputData string
	var source string

	switch {
	case hasPipe:
		inputData = pipeData
		source = "pipe"
	case filePath != "":
		// Expand ~
		filePath = expandHome(filePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		inputData = string(data)
		source = filePath
	default:
		// Read from clipboard
		content, err := clipboard.ReadAll()
		if err != nil || strings.TrimSpace(content) == "" {
			return fmt.Errorf("clipboard is empty. Use -file <path>, pipe input, or copy JSON to clipboard")
		}
		inputData = content
		source = "clipboard"
	}

	inputData = strings.TrimSpace(inputData)
	if inputData == "" {
		return fmt.Errorf("input is empty")
	}

	// Execute the selected mode
	switch mode {
	case "format":
		return j.doFormat(inputData, sortKeys, raw)
	case "compress":
		return j.doCompress(inputData, sortKeys, true)
	case "escape":
		return j.doEscape(inputData)
	case "unescape":
		return j.doUnescape(inputData)
	case "interactive":
		return j.doInteractive(inputData, sortKeys, source)
	default:
		return j.doInteractive(inputData, sortKeys, source)
	}
}

// doFormat pretty-prints JSON with optional color and key sorting.
func (j *Jv) doFormat(input string, sortKeys bool, raw bool) error {
	data, err := DecodeJSON(input)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if sortKeys {
		if om, ok := data.(*OrderedMap); ok {
			om.SortKeys()
		}
	}

	output := FormatJSON(data, 2, !raw)
	fmt.Println(output)
	return nil
}

// doCompress minifies JSON to a single line.
func (j *Jv) doCompress(input string, sortKeys bool, escape bool) error {
	data, err := DecodeJSON(input)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if sortKeys {
		if om, ok := data.(*OrderedMap); ok {
			om.SortKeys()
		}
	}

	output := CompactJSON(data, escape)
	fmt.Println(output)
	return nil
}

// doEscape converts non-ASCII characters to \uXXXX escapes.
func (j *Jv) doEscape(input string) error {
	data, err := DecodeJSON(input)
	if err != nil {
		// If it's not valid JSON, just escape the raw string
		fmt.Println(EscapeUnicode(input))
		return nil
	}
	output := CompactJSON(data, true)
	fmt.Println(output)
	return nil
}

// doUnescape converts \uXXXX sequences back to UTF-8.
func (j *Jv) doUnescape(input string) error {
	fmt.Println(UnescapeUnicode(input))
	return nil
}

// doInteractive launches the interactive tree viewer.
func (j *Jv) doInteractive(input string, sortKeys bool, source string) error {
	data, err := DecodeJSON(input)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if sortKeys {
		if om, ok := data.(*OrderedMap); ok {
			om.SortKeys()
		}
	}

	return RunInteractive(data, source)
}

// printHelp displays usage information.
func (j *Jv) printHelp() {
	fmt.Printf("jv - JSON Viewer & Formatter v%s\n\n", j.version)
	fmt.Println("Usage:")
	fmt.Println("  v jv [flags]           Read from clipboard")
	fmt.Println("  v jv -file <path>      Read from file")
	fmt.Println("  echo '{...}' | v jv    Read from pipe/stdin")
	fmt.Println()
	fmt.Println("Modes:")
	fmt.Println("  (default)  Interactive tree viewer (browse, fold/unfold, copy)")
	fmt.Println("  -f         Format (pretty-print) JSON")
	fmt.Println("  -c         Compress (minify) JSON")
	fmt.Println("  -e         Escape non-ASCII to \\uXXXX")
	fmt.Println("  -u         Unescape \\uXXXX to UTF-8")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -sort      Sort object keys alphabetically")
	fmt.Println("  -raw       Disable colored output (with -f)")
	fmt.Println("  -file      Read from file path")
	fmt.Println("  -h         Show this help")
	fmt.Println()
	fmt.Println("Interactive viewer keys:")
	fmt.Println("  Enter      Collapse/expand current node")
	fmt.Println("  ↑↓ / jk    Navigate up/down")
	fmt.Println("  c          Collapse all")
	fmt.Println("  e          Expand all")
	fmt.Println("  y          Copy current node value to clipboard")
	fmt.Println("  p          Copy current node path to clipboard")
	fmt.Println("  q          Quit")
}

// expandHome expands ~ to the user's home directory.
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
