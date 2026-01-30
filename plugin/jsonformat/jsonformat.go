package plugin_jsonformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/atotto/clipboard"
)

type JsonFormat struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (j *JsonFormat) Init() error {
	j.name = "jsonformat"
	j.version = "0.0.1"
	j.description = "Format and beautify JSON strings"
	j.command = "jsonformat"
	j.args = map[string]string{
		"-c":  "JSON content string (supports pipe mode)",
		"-cp": "Format JSON and copy to clipboard",
		"-i":  "Input file path",
		"-o":  "Output file path (default: stdout)",
	}
	j.author = "vst"
	return nil
}

func (j *JsonFormat) GetName() string {
	return j.name
}

func (j *JsonFormat) GetVersion() string {
	return j.version
}

func (j *JsonFormat) GetDescription() string {
	return j.description
}

func (j *JsonFormat) GetCommand() string {
	return j.command
}

func (j *JsonFormat) GetArgs() map[string]string {
	return j.args
}

func (j *JsonFormat) GetAuthor() string {
	return j.author
}

func formatJSON(content string) (string, error) {
	var jsonObj interface{}
	decoder := json.NewDecoder(bytes.NewReader([]byte(content)))
	decoder.UseNumber()
	if err := decoder.Decode(&jsonObj); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	formatted, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}

	return string(formatted), nil
}

func (j *JsonFormat) Run(args []string) error {
	var content string
	var outputPath string
	shouldCopy := false

	// Parse arguments
	for key, arg := range args {
		if arg == "-c" && len(args) > key+1 {
			content = args[key+1]
		}
		if arg == "-cp" {
			shouldCopy = true
		}
		if arg == "-i" && len(args) > key+1 {
			outputPath = args[key+1]
		}
		if arg == "-o" && len(args) > key+1 {
			outputPath = args[key+1]
		}
	}

	// Handle single argument (pipe mode or direct input)
	if len(args) == 1 && content == "" {
		content = args[0]
	}

	// If no content, read from stdin
	if content == "" {
		stat, _ := os.Stdin.Stat()
		if stat.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
			bytes, _ := io.ReadAll(os.Stdin)
			content = string(bytes)
		}
	}

	if content == "" {
		return fmt.Errorf("no JSON input provided. Use -c for content, -i for file, or pipe JSON")
	}

	// Format JSON
	formatted, err := formatJSON(content)
	if err != nil {
		return err
	}

	// Copy to clipboard if requested
	if shouldCopy {
		if err := clipboard.WriteAll(formatted); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("Formatted JSON copied to clipboard")
	}

	// Output to file
	if outputPath != "" {
		err := os.WriteFile(outputPath, []byte(formatted), 0644)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		fmt.Printf("Formatted JSON saved to %s\n", outputPath)
		return nil
	}

	// Default: print to stdout
	fmt.Println(formatted)
	return nil
}

func (j *JsonFormat) Stop() error {
	return nil
}
