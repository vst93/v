package plugin_jv

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
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
		"-f":    "Format (pretty-print) JSON from clipboard/file/pipe/stdin",
		"-c":    "Compress (minify) JSON to a single line",
		"-e":    "Escape non-ASCII to \\uXXXX sequences",
		"-u":    "Unescape \\uXXXX sequences to UTF-8 text",
		"-sort": "Sort object keys alphabetically",
		"-tui":  "Interactive tree viewer (default when no mode given; -i is a synonym)",
		"-file": "Read from file path instead of clipboard",
		"-url":  "Read from URL (HTTP/HTTPS)",
		"-clip": "Read from clipboard (the default source)",
		"-pipe": "Read from pipe/stdin (auto-detected)",
		"-out":  "Write the result to a file instead of stdout",
		"-copy": "Copy the result to clipboard",
		"-raw":  "Disable colored output (plain text)",
		"-h":    "Show help",
	}
	j.author = "vst"
	return nil
}

func (j *Jv) GetName() string            { return j.name }
func (j *Jv) GetVersion() string         { return j.version }
func (j *Jv) GetDescription() string     { return j.description }
func (j *Jv) GetCommand() string         { return j.command }
func (j *Jv) GetArgs() map[string]string { return j.args }
func (j *Jv) GetAuthor() string          { return j.author }

func (j *Jv) Stop() error { return nil }

func (j *Jv) Run(args []string) error {
	// Parse flags
	var (
		mode     string // "format", "compress", "escape", "unescape", "interactive"
		filePath string
		url      string
		outPath  string
		raw      bool
		sortKeys bool
		toClip   bool
		pipeData string
		hasPipe  bool
	)

	mode = "interactive" // default

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-f":
			mode = "format"
		case "-c":
			mode = "compress"
		case "-e":
			mode = "escape"
		case "-u":
			mode = "unescape"
		case "-tui", "-i":
			mode = "interactive"
		case "-sort":
			sortKeys = true
		case "-raw":
			raw = true
		case "-copy":
			toClip = true
		case "-clip":
			// Clipboard is already the default source; accepted for symmetry
			// with the other plugins.
		case "-file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		case "-url":
			if i+1 < len(args) {
				url = args[i+1]
				i++
			}
		case "-out":
			if i+1 < len(args) {
				outPath = args[i+1]
				i++
			}
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
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
	case url != "":
		data, err := fetchURL(url)
		if err != nil {
			return err
		}
		inputData = data
		source = url
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
	if mode == "interactive" {
		return j.doInteractive(inputData, sortKeys, source)
	}

	// Colored output would leak ANSI escapes into a file or the clipboard,
	// so those destinations always get plain text.
	if outPath != "" || toClip {
		raw = true
	}

	var output string
	var err error
	switch mode {
	case "format":
		output, err = j.doFormat(inputData, sortKeys, raw)
	case "compress":
		output, err = j.doCompress(inputData, sortKeys, true)
	case "escape":
		output, err = j.doEscape(inputData)
	case "unescape":
		output = UnescapeUnicode(inputData)
	}
	if err != nil {
		return err
	}

	return emit(output, outPath, toClip)
}

// emit writes the result to stdout, and additionally to a file (-out) and/or
// the clipboard (-copy).
func emit(output, outPath string, toClip bool) error {
	if outPath != "" {
		if err := os.WriteFile(expandHome(outPath), []byte(output+"\n"), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
		fmt.Printf("✅ Written to %s\n", outPath)
	} else {
		fmt.Println(output)
	}

	if toClip {
		if err := clipboard.WriteAll(output); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✅ Copied to clipboard")
	}
	return nil
}

// doFormat pretty-prints JSON with optional color and key sorting.
func (j *Jv) doFormat(input string, sortKeys bool, raw bool) (string, error) {
	data, err := DecodeJSON(input)
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	if sortKeys {
		if om, ok := data.(*OrderedMap); ok {
			om.SortKeys()
		}
	}

	return FormatJSON(data, 2, !raw), nil
}

// doCompress minifies JSON to a single line.
func (j *Jv) doCompress(input string, sortKeys bool, escape bool) (string, error) {
	data, err := DecodeJSON(input)
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	if sortKeys {
		if om, ok := data.(*OrderedMap); ok {
			om.SortKeys()
		}
	}

	return CompactJSON(data, escape), nil
}

// doEscape converts non-ASCII characters to \uXXXX escapes. Input that is not
// valid JSON is escaped as a raw string rather than treated as an error.
func (j *Jv) doEscape(input string) (string, error) {
	data, err := DecodeJSON(input)
	if err != nil {
		return EscapeUnicode(input), nil
	}
	return CompactJSON(data, true), nil
}

// doInteractive launches the interactive viewer. Invalid JSON is not
// an error: the viewer shows it as plain editable text without
// formatting.
func (j *Jv) doInteractive(input string, sortKeys bool, source string) error {
	return RunInteractive(input, source, sortKeys)
}

// printHelp displays usage information.
func (j *Jv) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>jv - JSON Viewer & Formatter v%s</>\n\n", j.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v jv [flags]           Read from clipboard (default)")
	color.Println("  v jv <green>-file</> <path>      Read from file")
	color.Println("  echo '{...}' | v jv    Read from pipe/stdin")
	color.Println()
	color.Println("<fg=magenta;op=bold>Modes:</>")
	color.Println("  (default)  Interactive tree viewer (browse, fold/unfold, copy, edit)")
	color.Println("  <green>-f</>         Format (pretty-print) JSON")
	color.Println("  <green>-c</>         Compress (minify) JSON")
	color.Println("  <green>-e</>         Escape non-ASCII to \\uXXXX")
	color.Println("  <green>-u</>         Unescape \\uXXXX to UTF-8")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-sort</>   Sort object keys alphabetically")
	color.Println("  <green>-raw</>    Disable colored output (with -f)")
	color.Println()
	color.Println("<gray>I/O: -pipe (auto) · -file <path> · -url <url> · -clip · -out <path> · -copy · -h</>")
	color.Println("<gray>     Priority: pipe > -file > -url > clipboard</>")
	color.Println()
	color.Println("<gray>Non-JSON input is opened as plain editable text.</>")
	color.Println("<gray>Press ? inside the viewer for the full key reference.</>")
	color.Println("<gray>--------------------------------------------------</>")
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

// fetchURL performs an HTTP GET and returns the response body.
func fetchURL(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(data), nil
}
