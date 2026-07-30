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
make build          # or: go build -o v .
./v -h
./v -version        # "dev" for local builds, the release tag for published ones
```

## Flag Conventions

The same flag means the same thing in every plugin:

| Flag | Meaning |
|------|---------|
| `-pipe` | Read from pipe/stdin (auto-detected, you rarely type it) |
| `-file <path>` | Read input from a file |
| `-clip` | **Read** the clipboard as input |
| `-url <url>` | Read input from a URL |
| `-out <path>` | Write the result to a file |
| `-copy` | **Write** the result to the clipboard |
| `-tui` | Interactive TUI mode |
| `-raw` | Plain text output, no colors |
| `-h` | Show help |

Input priority: `-pipe` > `-file` > `-url` > positional argument > clipboard.

Two documented exceptions: `jv` keeps its short mode letters (`-f` `-c` `-e`
`-u` `-i`), and `diff` uses `-left`/`-right` because it takes two inputs.

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
$ v json2excel -file 'data/input.json' -k 'data.list'

# Convert JSON string directly
$ v json2excel '[{"name":"张三","age":25},{"name":"李四","age":30}]'

# Use pipe input
$ curl -s 'https://api.example.com/data' | v json2excel -k 'items'

# Specify output path
$ v json2excel -file 'data.json' -out '/custom/path/output.xlsx'

# Keep nested JSON as strings (don't expand columns)
$ v json2excel -file 'data.json' -unexpand
```

**json2excel options:**

| Option | Description |
|--------|-------------|
| `-file <path>` | Input JSON file path |
| `-out <path>` | Output file path (defaults to ~/Downloads) |
| `-k` | Drill down key, use dot separator (e.g., `-k data.list.items`) |
| `-unexpand` | Don't expand nested JSON objects to multiple columns |
| `-pipe` | Read JSON from stdin/pipe (auto-detected) |
| `-h` | Show help |

Inline JSON is passed as a positional argument. Input priority: pipe > `-file` > argument.

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
$ v gp -l 16 -copy

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
| `-nl` | Exclude lowercase letters |
| `-nu` | Exclude uppercase letters |
| `-nd` | Exclude digits |
| `-ns` | Exclude special characters |
| `-copy` | Copy to clipboard (non-interactive) |
| `-tui` | Force interactive TUI mode |
| `-h` | Show help |

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

# Read from URL
$ v jv -url https://api.example.com/data

# Format a file and copy the result
$ v jv -f -file data.json -copy

# Format a file and write the result somewhere else
$ v jv -f -file data.json -out pretty.json

# Pipe input
$ cat data.json | v jv
```

**Modes:** `-f` format · `-c` compress · `-e` escape to `\uXXXX` · `-u` unescape ·
`-tui` (or `-i`) interactive viewer, which is the default.

**Options:** `-sort` sort keys · `-raw` no colors · `-file <path>` · `-url <url>` ·
`-clip` · `-out <path>` · `-copy` · `-h`.

`jv` keeps its short mode letters for historical reasons — note that here `-c`
means *compress*, not *copy*. Use `-copy` to copy. Writing to `-out` or `-copy`
always produces plain text, never ANSI colors.

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
| edit: `Shift`+arrows | Select text (`Ctrl-A` select all, `Ctrl-C`/`X`/`V` copy/cut/paste) |
| edit: `Ctrl`+`←`/`→` | Move by word |
| `f` | Reformat document |
| `/` or `Ctrl-F` | Search panel (`Enter` next, `Shift-Enter` prev, `Alt-C` case, `Alt-W` word, `Alt-R` regex) |
| `n` / `N` / `F3` | Next / previous match |
| `Tab` | Filter bar: `.key` `[0]` `["k"]` `.length` `.map(.k)` |
| `u` | Toggle `\uXXXX` display |
| `F` / `M` / `E` | Copy formatted / minified / minified+`\uXXXX` escaped JSON |
| `y` / `p` | Copy value / path at cursor |
| wheel | Scroll view (touchpad-smooth; cursor stays put) |
| click | Select line; double-click selects word & edits; click gutter chevron folds; drag to select text; drag scrollbar |
| `?` / `q` | Help / quit |

### diff - Side-by-side Diff Viewer

Interactive text comparison tool with word-level inline diff highlighting.
Supports file comparison, clipboard comparison, pipe input, and a paste mode
for quick ad-hoc diffs.

```bash
# Compare two files (interactive TUI)
$ v diff -left file1.txt -right file2.txt

# Compare file vs clipboard
$ v diff -left file.txt -clip

# Compare pipe input vs file
$ echo 'hello' | v diff -right file.txt

# Paste mode: paste left/right text directly in the TUI
$ v diff

# Inline unified diff output (no TUI)
$ v diff -left a.txt -right b.txt -inline
```

**Options:**

| Option | Description |
|--------|-------------|
| `-left <path>` | Left file path |
| `-right <path>` | Right file path |
| `-clip` | Read clipboard as a source |
| `-pipe` | Read from stdin/pipe (auto-detected, used as left side) |
| `-inline` | Output unified diff to stdout (no TUI) |
| `-raw` | Plain text output (with `-inline`, no colors) |
| `-h` | Show help |

**Diff viewer keys:**

| Key | Action |
|-----|--------|
| `↑↓` / `j` `k` | Navigate up/down |
| `n` / `N` | Next / previous diff hunk |
| `/` | Search (type term, Enter to jump) |
| `c` | Show only changed lines |
| `a` | Show all lines |
| `e` | Edit (return to paste mode) |
| `q` | Quit |

**Paste mode keys:**

| Key | Action |
|-----|--------|
| `Tab` | Switch left/right panel |
| `Ctrl-A` | Select all |
| `Ctrl-C` / `X` / `V` | Copy / cut / paste |
| `Ctrl-Z` / `Y` | Undo / redo |
| `Shift`+arrows | Select text |
| `Ctrl`+`←`/`->` | Move by word |
| `Ctrl-D` | Compute diff |
| `Esc` | Quit |

### cp - Copy to Clipboard

Copy text to clipboard. Designed for pipe mode to chain with other commands.

```bash
# Copy piped stdin to clipboard
$ echo "Hello World" | v cp

# Copy argument directly
$ v cp "some text"

# Trim whitespace before copying
$ printf '  \n  Hello  \n  ' | v cp -trim
# copies "Hello"

# Trim one side only
$ echo "  hello  " | v cp -triml   # copies "hello  "
$ echo "  hello  " | v cp -trimr   # copies "  hello"
```

**Options:**

| Option | Description |
|--------|-------------|
| `-trim` | Trim leading & trailing whitespace (spaces, tabs, newlines) |
| `-triml` | Trim leading whitespace only |
| `-trimr` | Trim trailing whitespace only |
| `-pipe` | Read from pipe/stdin (auto-detected) |
| `-h` | Show help |

Input priority: pipe > argument > clipboard.

### codec - Encode/Decode Utility

Encode and decode text using Base64, Base32, URL, Hex, HTML, or Unicode encoding.

**Short command:** `cc` (`v enc` also still works — `codec` is the former `enc`)

```bash
# Interactive TUI (no arguments)
$ v codec

# Base64 encode
$ v codec -b64 "Hello World"
SGVsbG8gV29ybGQ=

# Base64 decode
$ v cc -b64d "SGVsbG8gV29ybGQ="
Hello World

# URL encode
$ v codec -url "hello world&foo=bar"
hello+world%26foo%3Dbar

# Hex encode
$ v codec -hex "Hello"
48656c6c6f

# HTML escape
$ v codec -html '<a href="x">test</a>'
&lt;a href=&#34;x&#34;&gt;test&lt;/a&gt;

# Unicode escape (supports emoji via surrogate pairs)
$ v codec -uni "你好"
\u4f60\u597d

# Pipe input
$ echo "Hello" | v codec -b64
SGVsbG8K

# Copy result to clipboard / write it to a file
$ v codec -b64 "secret" -copy
$ v codec -b64 "secret" -out secret.txt
```

**Modes:**

| Option | Description | Option | Description |
|--------|-------------|--------|-------------|
| `-b64` | Base64 encode | `-b64d` | Base64 decode |
| `-b32` | Base32 encode | `-b32d` | Base32 decode |
| `-url` | URL encode (percent-encoding) | `-urld` | URL decode |
| `-hex` | Hex encode | `-hexd` | Hex decode |
| `-html` | HTML escape | `-htmld` | HTML unescape |
| `-uni` | Unicode escape (non-ASCII to `\uXXXX`) | `-unid` | Unicode unescape |

**Options:**

| Option | Description |
|--------|-------------|
| `-file <path>` | Read from file path |
| `-clip` | Read from clipboard |
| `-pipe` | Read from pipe/stdin (auto-detected) |
| `-out <path>` | Write the result to a file |
| `-copy` | Copy result to clipboard |
| `-tui` | Interactive TUI (also the default with no mode) |
| `-h` | Show help |

Input priority: pipe > file > argument > clipboard.

**Interactive TUI** (`v codec` with no arguments) — fully mouse and touch driven:

| Key / Mouse | Action |
|-------------|--------|
| click / tap | Select a codec or mode, place the cursor, press a button |
| drag | Select text in the input area |
| wheel | Scroll input or output |
| `Tab` / `Shift+Tab` | Cycle focus: Input → Codec → Mode → Output → buttons |
| `↑` `↓` | Change selection (when a list is focused) |
| `1`-`6` | Quick-select codec (when the Codec list is focused) |
| `e` / `d` | Encode / decode (when the Mode list is focused) |
| `Ctrl-C` / `Ctrl-X` / `Ctrl-V` | Copy / cut / paste in the input area (system clipboard) |
| `Ctrl-Z` / `Ctrl-Y` | Undo / redo in the input area |
| `y` / `Ctrl-Y` | Copy the output to clipboard |
| `Ctrl-R` | Swap: feed the output back in and flip encode↔decode |
| `Ctrl-L` | Clear the input |
| `Esc` | Quit |

Conversion is live — the output updates as you type. Decode errors appear in the
output box instead of clearing it silently. Below 64 columns the Codec/Mode
column moves above the editor so nothing gets clipped.

### gencm - Generate Commit Message

Generate a Conventional Commits message from your git changes using an
OpenAI-compatible AI model.

**Short command:** `gc` (alias for `gencm`)

```bash
# Generate from staged changes (git diff --cached)
$ v gencm
📦 Generating commit message (gpt-4o-mini)...

feat(gcm): add AI commit message generator

# Stage all changes then generate
$ v gencm -add

# Generate and copy to clipboard (skip menu)
$ v gencm -copy

# Use unstaged changes (git diff)
$ v gencm -u

# Use all changes vs HEAD (git diff HEAD)
$ v gencm -a

# Generate for a project in another directory
$ v gencm -C ~/projects/myapp

# Read the diff from a pipe instead of running git
$ git diff --cached | v gencm
```

**Options:**

| Option | Description |
|--------|-------------|
| `-add` | Stage all changes (`git add .`) before generating |
| `-u` | Use unstaged changes (`git diff`) |
| `-a` | Use all changes vs HEAD (`git diff HEAD`) |
| `-C <path>` | Run as if git was started in `<path>` (default: current directory) |
| `-lang` | Commit message language: `en`, `zh`, or custom text (default: `en`) |
| `-pipe` | Read diff from stdin/pipe instead of running git (auto-detected) |
| `-copy` | Copy to clipboard (skips interactive menu in non-pipe mode) |
| `-h` | Show help |

**Configuration** — add a `[gcm]` section to `~/.v_tools/settings.ini`:

```ini
[gcm]
api_key = sk-xxx
base_url = https://api.openai.com/v1
model = gpt-4o-mini
lang = en
```

`base_url` and `model` are optional and default to the values above. Any
OpenAI-compatible endpoint works. Diffs longer than 50000 characters are
condensed per-file before the API call to stay within the model's token
limit. Large changesets preserve file-level structure: every file's
header and hunk headers survive, only verbose content within each file
is trimmed. A `git diff --stat` summary is always included so the model
sees the full scope even when the diff is condensed.

**Commit message language** - `lang` controls the output language:

| Value | Effect |
|------|--------|
| `en` | English (default) |
| `zh` | Chinese |
| (any text) | Custom instruction, e.g. `Write in Japanese` |

On first run in interactive mode, you'll be prompted to pick. Override at
any time with `-lang`:

```bash
$ v gencm -lang zh        # Chinese
$ v gencm -lang "Write in Japanese"
```

**Interactive menu** (non-pipe mode only): after generating a commit message,
choose an action:

| Choice | Action |
|--------|--------|
| `1` or `Enter` | Copy to clipboard (default) |
| `2` | Commit with the message |
| `3` | Commit and push |
| `4` | Do nothing |

Pipe mode and the `-copy` flag skip the menu — they just print the message
(or print + copy).

## Development

### Project Structure

```
v/
├── main.go              # Entry point and plugin orchestration
├── Makefile             # build / test / release targets (owns the version ldflags)
├── cmd/
│   └── install.sh       # Installation script for Linux/macOS/Termux
├── service/
│   ├── plugin.go        # Plugin registry and PluginTemplate interface
│   └── help.go          # Help() output; VVersion (injected at link time)
├── plugin/
│   ├── pwd/             # pwd command implementation
│   ├── tt/              # timestamp converter implementation
│   ├── json2excel/      # JSON to Excel converter
│   ├── translate/       # Translation service
│   ├── genpwd/          # Random password generator
│   ├── jv/              # JSON viewer
│   ├── diff/            # Side-by-side diff viewer
│   ├── cp/              # Copy to clipboard (pipe-friendly)
│   ├── codec/           # Encode/Decode utility (base64, url, hex, html, unicode)
│   └── gcm/             # AI commit message generator (command: gencm)
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
4. Follow the flag conventions above — in particular, always handle `-h`

### Build Commands

The version reported by `v -version` is injected at link time. It is `dev` for
local builds and the release tag for published binaries.

```bash
make build                    # dev build
make build VERSION=0.0.6      # inject a specific version
make test                     # go vet ./... && go test ./...
make release VERSION=0.0.6    # cross-compile all platforms into ./output
make help                     # list targets

go build -o v .               # plain build; reports "dev"
```

Note the build target is `.`, not `main.go` — the root package spans several files.

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## License

GNU GPL v3 - see LICENSE file for details.

## Author

Created by vst93
