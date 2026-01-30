# AGENTS.md

This file provides guidelines for agentic coding agents operating in this repository.

## Project Overview

**v** is a command-line utility tool built with Go, featuring a plugin-based architecture for extending functionality. It provides various developer tools like timestamp conversion, JSON to Excel export, clipboard utilities, and text translation.

- **Language**: Go 1.24.3
- **Architecture**: Plugin-based CLI application
- **Platforms**: macOS, Linux, Windows, Android (cross-compiled)
- **Repository**: https://github.com/vst93/v

## Build, Lint, and Test Commands

### Build Commands

```bash
# Build for current platform
go build -o v main.go

# Build for all platforms (CI uses GitHub Actions workflow)
GOOS=darwin GOARCH=amd64 go build -o v-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o v-darwin-arm64 main.go
GOOS=linux GOARCH=amd64 go build -o v-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o v.exe main.go
GOOS=android GOARCH=amd64 CC="x86_64-linux-android32-clang" CGO_ENABLED=1 go build -o v-android-amd64 main.go
GOOS=android GOARCH=arm64 CC="aarch64-linux-android32-clang" CGO_ENABLED=1 go build -o v-android-arm64 main.go

# Build with optimization flags (used in release workflow)
go build -ldflags "-w -s" -o v main.go
go build -trimpath -ldflags "-w -s" -o v main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test -v ./plugin/json2excel/...

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### Linting

```bash
# Run go vet
go vet ./...

# Format code
gofmt -w .

# Check for unused imports
goimports -w .
```

### Module Management

```bash
# Download dependencies
go mod download

# Tidy go.mod
go mod tidy

# Verify dependencies
go mod verify
```

## Code Organization

```
v/
├── main.go              # Entry point, handles argument parsing and plugin routing
├── service/
│   ├── plugin.go        # PluginTemplate interface and registry (PLUGIN REGISTRATION)
│   └── help.go          # Help text generation with colored output
├── plugin/
│   ├── pwd/             # Print working directory with clipboard copy
│   ├── tt/              # Timestamp converter (Unix seconds/milliseconds)
│   ├── json2excel/      # JSON to Excel converter with key drilling
│   ├── translate/       # Text translation (Google, CNKI)
│   └── template/        # Plugin template for new plugins
├── setting/
│   └── ini.go           # Configuration management using gookit/ini
├── cmd/
│   └── install.sh       # Homebrew/Linux/macOS installation script
├── go.mod               # Module definition
└── AGENTS.md            # This file
```

### Key Entry Points

- **main.go:11-49**: Main entry point. Loads settings, detects pipe mode, routes commands to plugins
- **service/plugin.go:56-67**: Plugin registry - ALL plugins must be registered here
- **setting/ini.go:11-32**: Initializes `~/.v_tools/settings.ini` configuration file

## Code Style Guidelines

### General Principles

- Follow standard Go conventions (effectivego.com)
- Keep functions short and focused
- Use meaningful variable and function names
- Handle errors explicitly, never ignore them with `_`
- Use `strings.Builder` for string concatenation in loops (avoid `+=`)

### Imports

- Use grouped imports with standard library first
- Use aliased imports for clarity when package names conflict
- Example:
  ```go
  import (
      "fmt"
      "os"
      "v/service"
      "v/setting"
  )
  ```

### Naming Conventions

- **Packages**: lowercase, concise, no underscores
- **Types**: PascalCase (e.g., `Json2Excel`, `PluginTemplate`)
- **Variables**: camelCase (e.g., `inputPath`, `keyDrill`)
- **Constants**: camelCase or SCREAMING_SNAKE_CASE for exported constants
- **Acronyms**: Keep consistent casing (e.g., `JSONProcessor`, not `JsonProcessor`)
- **Plugin packages**: Use underscore format matching directory (e.g., `plugin_pwd` for `plugin/pwd/`)

### Error Handling

- Return errors as values, never use `panic` except for unrecoverable initialization errors
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`
- Check errors immediately after calling functions
- Example:
  ```go
  err := processData(content)
  if err != nil {
      return fmt.Errorf("failed to process data: %w", err)
  }
  ```

## Plugin Architecture

### PluginTemplate Interface

All plugins MUST implement this interface defined in `service/plugin.go:10-21`:

```go
type PluginTemplate interface {
    Init() error           // Initialize plugin metadata (name, version, description, etc.)
    Run(args []string) error  // Execute plugin logic
    Stop() error           // Cleanup (can be no-op if no cleanup needed)
    
    GetName() string
    GetVersion() string
    GetDescription() string
    GetCommand() string
    GetArgs() map[string]string
    GetAuthor() string
}
```

### Plugin Registration

**CRITICAL**: After creating a new plugin, you MUST register it in `service/plugin.go:56-67`:

```go
func (p Plugin) List() []PluginTemplate {
    list := []PluginTemplate{
        &(plugin_json2excel.Json2Excel{}),
        &(plugin_pwd.Pwd{}),
        &(plugin_tt.TT{}),
        &(plugin_translate.Tr{}),
        // Add your new plugin here
    }
    for _, v := range list {
        v.Init()
    }
    return list
}
```

### Plugin Template

Use `plugin/template/template.go` as a template for new plugins:

```go
package plugin_template

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
    t.description = "Description of your plugin"
    t.command = "template"  // Command users will type: v template
    t.args = map[string]string{
        "-v": "description of flag",
    }
    t.author = "vst"
    return nil
}

func (t *Template) Run(args []string) error {
    // Implement plugin logic
    return nil
}

func (t *Template) Stop() error {
    return nil
}

// Implement all Get* methods (GetName, GetVersion, etc.)
```

### Struct Design

- Use embedded types when composition is appropriate
- Initialize structs with field names for clarity
- Example:
  ```go
  type Json2Excel struct {
      name        string
      version     string
      description string
      command     string
      args        map[string]string
      author      string
  }
  ```

## Command-Line Argument Patterns

### Supported Patterns

- Use flag package or manual parsing as seen in existing plugins
- Support `-h`/`--help` for all plugins
- Support pipe mode via stdin detection (auto-detected in main.go:22-27)
- Example pipe detection in main.go:
  ```go
  stat, _ := os.Stdin.Stat()
  if stat.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
      bytes, _ := io.ReadAll(os.Stdin)
      args = append(args, "-pipe")
      args = append(args, string(bytes))
  }
  ```

### Argument Parsing Approach

Plugins use manual index-based parsing (see `plugin/json2excel/json2excel.go:61-80`):

```go
for key, arg := range args {
    if arg == "-i" && len(args) > key+1 {
        inputPath = args[key+1]
    }
    if arg == "-o" && len(args) > key+1 {
        outputPath = args[key+1]
    }
}
```

## Color Output

Use `github.com/gookit/color` for colored terminal output (see `service/help.go`):

```go
import "github.com/gookit/color"

// Usage examples:
color.New(color.FgCyan, color.Bold).Sprint("text")
color.New(color.FgGreen).Sprint("Version: " + version)
```

## Configuration Management

- Settings are stored in `~/.v_tools/settings.ini`
- Use `gookit/ini/v2` for configuration (see `setting/ini.go`)
- Key functions:
  - `InitSetting()`: Initialize config directory and file
  - `Set(section, key, value string) error`: Set a configuration value
  - `SaveSetting()`: Persist settings to file

## Documentation

- Document all exported types and functions
- Use doc comments starting with type/function name
- Example:
  ```go
  // Json2Excel converts JSON data to Excel files
  type Json2Excel struct { ... }
  
  // Run executes the JSON to Excel conversion
  func (j *Json2Excel) Run(args []string) error { ... }
  ```

## Performance Considerations

- Reuse buffers where appropriate
- Close resources immediately after use (files, connections)
- Use appropriate data structures (map for lookups, slices for ordered data)
- Use `strings.Builder` instead of string concatenation in loops

## Dependencies

- Keep dependencies minimal
- Current key dependencies:
  - `github.com/gookit/ini/v2`: Configuration management
  - `github.com/gookit/color`: Terminal color output
  - `github.com/xuri/excelize/v2`: Excel file generation
  - `github.com/atotto/clipboard`: Clipboard operations
  - `golang.org/x/text`: Text processing utilities

All third-party imports must be in go.mod. Run `go mod tidy` after adding dependencies.

## Release and CI/CD

- **CI/CD**: GitHub Actions workflow in `.github/workflows/release.yml`
- **Release triggers**: On version tag creation
- **Build matrix**: linux/windows/darwin/android × amd64/arm64
- **Artifact upload**: ZIP files with SHA256 checksums
- **Version management**: Go 1.24.3 (matches go.mod)

### Release Build Command Pattern

```bash
# From release workflow - uses CGO for Android
export CGO_ENABLED=$( [ "$GOOS" == "android" ] && echo 1 || echo 0 )
go build -o output/v -trimpath -ldflags "-w -s" .
```

## Gotchas and Common Issues

1. **Plugin Registration**: New plugins MUST be manually added to `service/plugin.go:56-67` in the `List()` function
2. **Pipe Mode Detection**: Main entry point auto-detects pipe input and passes as `-pipe` flag
3. **Error Wrapping**: Use `%w` verb with `fmt.Errorf` for error wrapping, not `%s`
4. **String Concatenation**: Use `strings.Builder` instead of `+=` in loops to avoid performance issues
5. **Package Naming**: Plugin package names use underscore format (e.g., `plugin_pwd` for directory `plugin/pwd/`)
6. **Import Aliases**: Plugin imports in `service/plugin.go` use aliases matching directory names
7. **Config Path**: Configuration is stored in user's home directory (`~/.v_tools/`), not in project directory
8. **No Panic in Plugins**: Plugins should return errors rather than panicking (except unrecoverable init errors)

## Development Workflow

1. Create new plugin directory under `plugin/`
2. Copy `plugin/template/template.go` as starting point
3. Implement all PluginTemplate interface methods
4. Add plugin import and registration in `service/plugin.go`
5. Test with `go build -o v main.go && ./v <command>`
6. Update README.md with command documentation if needed
