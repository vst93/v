package plugin_fx

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
)

type Fx struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (f *Fx) Init() error {
	f.name = "fx"
	f.version = "0.0.2"
	f.description = "Terminal JSON viewer & processor using fx"
	f.command = "fx"
	f.args = map[string]string{
		"-raw": "Pass raw JSON to fx without Unicode decoding",
	}
	f.author = "vst"
	return nil
}

func (f *Fx) GetName() string {
	return f.name
}

func (f *Fx) GetVersion() string {
	return f.version
}

func (f *Fx) GetDescription() string {
	return f.description
}

func (f *Fx) GetCommand() string {
	return f.command
}

func (f *Fx) GetArgs() map[string]string {
	return f.args
}

func (f *Fx) GetAuthor() string {
	return f.author
}

// isURL checks if the content is a URL
func isURL(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://")
}

// isFile checks if the content is a file path that exists
func isFile(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	// Expand home directory ~
	if strings.HasPrefix(content, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			content = home + content[1:]
		}
	}

	// Check if absolute path exists
	_, err := os.Stat(content)
	return err == nil
}

// isJSON checks if the content looks like JSON
func isJSON(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[")
}

// checkFxInstalled checks if fx is installed
func checkFxInstalled() bool {
	_, err := exec.LookPath("fx")
	return err == nil
}

// unescapeUnicode decodes Unicode escape sequences in JSON (e.g., \u4e2d\u6587 -> 中文)
func unescapeUnicode(content string) string {
	result := make([]byte, 0, len(content))
	i := 0
	for i < len(content) {
		if content[i] == '\\' && i+5 < len(content) && (content[i+1] == 'u' || content[i+1] == 'U') {
			var hex string
			if content[i+2] == '{' {
				end := strings.Index(content[i+3:], "}")
				if end == -1 {
					result = append(result, content[i])
					i++
					continue
				}
				hex = content[i+3 : i+3+end]
				i += end + 4
			} else {
				hex = content[i+2 : i+6]
				i += 6
			}

			var runeVal rune
			for _, c := range hex {
				var val int
				if c >= '0' && c <= '9' {
					val = int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					val = int(c - 'a' + 10)
				} else if c >= 'A' && c <= 'F' {
					val = int(c - 'A' + 10)
				} else {
					result = append(result, '\\')
					continue
				}
				runeVal = runeVal*16 + rune(val)
			}
			result = append(result, []byte(string(runeVal))...)
		} else {
			result = append(result, content[i])
			i++
		}
	}
	return string(result)
}

// runFxCommand executes the fx command with given input
func runFxCommand(input string, inputType string, raw bool) error {
	if !checkFxInstalled() {
		return fmt.Errorf("fx is not installed. Please install it first:\n" +
			"  macOS:    brew install fx\n" +
			"  Linux:    go install github.com/antonmedv/fx@latest\n" +
			"  Windows:  scoop install fx\n" +
			"  Or download from: https://github.com/antonmedv/fx/releases")
	}

	var cmd *exec.Cmd
	var jsonData []byte

	switch inputType {
	case "url":
		curlCmd := exec.Command("curl", "-s", input)
		data, err := curlCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to fetch URL: %w", err)
		}
		jsonData = data
	case "file":
		data, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		jsonData = data
	case "json":
		jsonData = []byte(input)
	default:
		return fmt.Errorf("unsupported input type: %s", inputType)
	}

	// Decode Unicode if not raw
	if !raw {
		jsonData = []byte(unescapeUnicode(string(jsonData)))
	}

	cmd = exec.Command("fx")
	cmd.Stdin = strings.NewReader(string(jsonData))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (f *Fx) Run(args []string) error {
	// Parse flags
	raw := false
	for _, arg := range args {
		if arg == "-raw" {
			raw = true
		}
	}

	// Read content from clipboard
	content, err := clipboard.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read from clipboard: %w", err)
	}

	content = strings.TrimSpace(content)

	if content == "" {
		return fmt.Errorf("clipboard is empty. Please copy some JSON content, URL, or file path to clipboard")
	}

	// Determine input type and run appropriate fx command
	var inputType string

	switch {
	case isURL(content):
		inputType = "url"
		fmt.Printf("Detected URL: %s\n", content)
	case isFile(content):
		inputType = "file"
		fmt.Printf("Detected file: %s\n", content)
	case isJSON(content):
		inputType = "json"
		fmt.Println("Detected JSON string")
	default:
		return fmt.Errorf("unsupported content type. Please copy one of the following:\n" +
			"  - A URL (http:// or https://)\n" +
			"  - A file path that exists\n" +
			"  - A JSON string (starts with { or [)")
	}

	fmt.Println("Running fx...")
	fmt.Println("---")

	return runFxCommand(content, inputType, raw)
}

func (f *Fx) Stop() error {
	return nil
}
