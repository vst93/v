# v - Gadgets under the terminal

A collection of useful command-line tools for developers. Built with Go, designed for productivity.

## Features

- **Cross-platform**: Works on macOS, Linux, Windows, and Android
- **Plugin architecture**: Easy to extend with new commands
- **Pipeline support**: Works with stdin/stdout for seamless integration
- **Clipboard integration**: Copy results with a single command

## Installation

### Homebrew (macOS)
```bash
# install
brew install vst93/tap/v

# uninstall
brew uninstall v
```

### Shell Script (Linux/macOS)
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/vst93/v/refs/heads/main/cmd/install.sh)"
```

### Manual Build
```bash
git clone https://github.com/vst93/v.git
cd v
go build -o v main.go
./v -h
```

## Available Commands

### pwd - Print Working Directory

Print the current directory path and automatically copy it to clipboard.

```bash
$ v pwd
📁 Current Directory:
/Users/vst/Code/goProgram/v
✅ Copied to clipboard
```

### tt - Timestamp Converter

Provides mutual conversion between timestamps and human-readable dates.

```bash
# Current Unix timestamp (seconds)
$ v tt
1641038400

# Current Unix timestamp (milliseconds)
$ v tt -m
1765533341652

# Convert date string to timestamp
$ v tt '2022-01-01 12:00:00'
1641038400

# Convert timestamp to date string
$ v tt 1641038400
2022-01-01 20:00:00
```

### json2excel - JSON to Excel Converter

Convert JSON data to Excel (.xlsx) files with support for nested objects and key drilling.

```bash
# Convert JSON file to Excel
$ v json2excel -i 'data/input.json' -k 'data.list'

# Convert JSON string directly
$ v json2excel -c '[{"name":"张三","age":25},{"name":"李四","age":30}]'

# Use pipe input
$ curl -s 'https://api.example.com/data' | v json2excel -k 'items'

# Specify output path
$ v json2excel -i 'data.json' -o '/custom/path/output.xlsx'

# Keep nested JSON as strings (don't expand columns)
$ v json2excel -i 'data.json' -unexpand
```

**json2excel options:**

| Option | Description |
|--------|-------------|
| `-i` | Input file path |
| `-c` | JSON content string (overrides `-i`) |
| `-o` | Output file path (defaults to ~/Downloads) |
| `-k` | Drill down key, use dot separator (e.g., `-k data.list.items`) |
| `-unexpand` | Don't expand nested JSON objects to multiple columns |

### tr - Text Translator

Translate text using Google Translate or CNKI (requires internet connection).

```bash
# Translate text
$ v tr 'Hello World'
你好，世界

# Enable/Disable Google Translate
$ v -enable-google
Google Translate enabled
$ v -disable-google
Google Translate disabled

# Enable/Disable CNKI Translate
$ v -enable-cnki
CNKI Translate enabled
$ v -disable-cnki
CNKI Translate disabled
```

### genpwd - Random Password Generator

Generate cryptographically secure random passwords with an interactive TUI for configuring rules.

**Short command:** `gp` (alias for `genpwd`)

```bash
# Interactive TUI (default) - configure rules and generate passwords
$ v gp

# Generate a 20-char password to stdout
$ v gp -l 20

# Generate 5 passwords of length 12
$ v gp -l 12 -n 5

# Generate and copy to clipboard
$ v gp -l 16 -c

# No special characters
$ v gp -l 16 -ns
```

**Interactive TUI keys:**

| Key | Action |
|-----|--------|
| `←` `→` | Adjust length (presets: 8 12 16 20 24 32 48 64) |
| `Enter` | Custom length input (on length row) |
| `Space` | Next preset / toggle checkbox |
| `Tab` / `↑↓` | Navigate between options |
| `r` | Regenerate password |
| `y` | Copy password to clipboard |
| `q` | Quit |

**Options:**

| Option | Description |
|--------|-------------|
| `-l N` | Password length (default 16) |
| `-n N` | Number of passwords (non-interactive, default 1) |
| `-c` | Copy to clipboard (non-interactive) |
| `-nl` | Exclude lowercase letters |
| `-nu` | Exclude uppercase letters |
| `-nd` | Exclude digits |
| `-ns` | Exclude special characters |
| `-i` | Force interactive TUI mode |

### jv - JSON Viewer

Editor-style JSON viewer (utools-like): line numbers, syntax highlighting, code folding,
search panel, path filter bar, smooth mouse/touchpad scrolling, and text editing with
graceful fallback for invalid JSON.

```bash
# Interactive viewer (from clipboard)
$ v jv

# Format (pretty-print) JSON
$ v jv -f

# Compress (minify) JSON
$ v jv -c

# Read from file
$ v jv -file data.json

# Pipe input
$ cat data.json | v jv
```

Non-JSON input opens as plain editable text without formatting. Editing the text back
into valid JSON automatically re-enables folding, path lookup and minified copy.

Viewer keys & mouse:

| Key / Mouse | Action |
|-------------|--------|
| `↑↓` / `j` `k` | Move cursor (`PgUp`/`PgDn` page, `g`/`G` first/last) |
| `Enter` / `Space` / `o` | Fold / unfold at cursor |
| `h` / `l` | Fold, jump to parent / unfold |
| `e` / `c` | Expand / collapse all |
| `i` | Inline edit (`Esc` done, `Ctrl-Z`/`Ctrl-Y` undo/redo) |
| `I` | Edit in `$EDITOR` (vim/vi/nano/notepad); returns on save |
| `f` | Reformat document |
| `/` or `Ctrl-F` | Search panel (`Enter` next, `Shift-Enter` prev, `Alt-C` case, `Alt-W` word, `Alt-R` regex) |
| `n` / `N` / `F3` | Next / previous match |
| `Tab` | Filter bar: `.key` `[0]` `["k"]` `.length` `.map(.k)` |
| `u` | Toggle `\uXXXX` display |
| `F` / `M` / `E` | Copy formatted / minified / minified+`\uXXXX` escaped JSON |
| `y` / `p` | Copy value / path at cursor |
| wheel | Scroll view (touchpad-smooth; cursor stays put) |
| click | Select line; double-click for quick inline edit; click gutter chevron folds; drag scrollbar |
| `?` / `q` | Help / quit |

### diff - Side-by-side Diff Viewer

Interactive text comparison tool with word-level inline diff highlighting.

```bash
# Compare two files (interactive TUI)
$ v diff -left file1.txt -right file2.txt

# Compare file vs clipboard
$ v diff -left file.txt -clip

# Inline unified diff output
$ v diff -left a.txt -right b.txt -inline
```

### fx - File Explorer

Quick file listing and exploration.

## Development

### Project Structure

```
v/
├── main.go              # Entry point and plugin orchestration
├── cmd/
│   └── install.sh       # Installation script for Linux/macOS
├── service/
│   └── plugin.go        # Plugin registry and PluginTemplate interface
├── plugin/
│   ├── pwd/             # pwd command implementation
│   ├── tt/              # timestamp converter implementation
│   ├── json2excel/      # JSON to Excel converter
│   ├── translate/       # Translation service
│   ├── genpwd/          # Random password generator
│   ├── jv/              # JSON viewer
│   ├── diff/            # Side-by-side diff viewer
│   └── fx/              # File explorer
└── setting/
    └── ini.go           # Configuration management
```

### Adding a New Plugin

1. Create a new directory under `plugin/`
2. Implement the `PluginTemplate` interface:

```go
type PluginTemplate interface {
    Init() error
    Run(args []string) error
    Stop() error
    GetName() string
    GetVersion() string
    GetDescription() string
    GetCommand() string
    GetArgs() map[string]string
    GetAuthor() string
}
```

3. Register your plugin in `service/plugin.go`

### Build Commands

```bash
# Build for current platform
go build -o v main.go

# Build for all platforms
GOOS=darwin GOARCH=amd64 go build -o v-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o v-darwin-arm64 main.go
GOOS=linux GOARCH=amd64 go build -o v-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o v.exe main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## License

MIT License - see LICENSE file for details.

## Author

Created by vst93
