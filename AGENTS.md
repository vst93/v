# AGENTS.md

This file provides guidelines for agentic coding agents operating in this repository.

## Build, Lint, and Test Commands

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

## Code Style Guidelines

### General Principles
- Follow standard Go conventions (effectivego.com)
- Keep functions short and focused
- Use meaningful variable and function names
- Handle errors explicitly, never ignore them with `_`

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

### Plugin Architecture
All plugins must implement the `PluginTemplate` interface:
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

### Documentation
- Document all exported types and functions
- Use doc comments starting with type/function name
- Example:
  ```go
  // Json2Excel converts JSON data to Excel files
  type Json2Excel struct { ... }
  
  // Run executes the JSON to Excel conversion
  func (j *Json2Excel) Run(args []string) error { ... }
  ```

### Command-Line Arguments
- Use flag package or manual parsing as seen in existing plugins
- Support `-h`/`--help` for all plugins
- Support pipe mode via stdin detection

### Code Organization
- **main.go**: Entry point and plugin orchestration
- **service/**: Core services and plugin registry
- **plugin/**: Plugin implementations in subdirectories
- **setting/**: Configuration and settings management

### Performance Considerations
- Reuse buffers where appropriate
- Close resources immediately after use (files, connections)
- Use appropriate data structures (map for lookups, slices for ordered data)

### Dependencies
- Keep dependencies minimal
- Use well-maintained packages (gookit/ini, xuri/excelize)
- All third-party imports should be in go.mod
