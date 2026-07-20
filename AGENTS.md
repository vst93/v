# Repository Guidelines

## Project Overview

**v** is a single-binary, plugin-based command-line utility ("gadgets under the terminal") built in Go. It bundles developer tools — timestamp conversion, JSON viewer/formatter, JSON→Excel export, text diff, password generation, translation, clipboard helpers — behind one `v <command>` dispatcher. Cross-compiled for macOS, Linux, Windows, and Android.

- **Module**: `v` (repo root is the main package)
- **Go version**: 1.24.3 (no `toolchain` directive)
- **Repo**: https://github.com/vst93/v
- **License**: GNU GPL v3 — see `LICENSE`. ⚠️ `README.md` incorrectly states "MIT License"; the LICENSE file is GPL v3. Treat GPL v3 as authoritative.

## Architecture & Data Flow

```
stdin/pipe ──► main.go ──► service.Plugin{}.List() ──► matched plugin.Run(args[1:]) ──► stdout / clipboard / TUI
                  │
                  ├─ setting.InitSetting()  (~/.v_tools/settings.ini)
                  ├─ pipe detection          (appends `-pipe <data>` to args)
                  ├─ alias map {"gp":"genpwd"}
                  └─ -h/-help/--help ──► service.Help()
```

**Dispatch flow** (`main.go`, 57 lines, `main()` at L11):
1. L15 `setting.InitSetting()` loads `~/.v_tools/settings.ini`.
2. L17-20 `args := os.Args[1:]`; if empty, defaults to `["-h"]`.
3. L21 `firstArg := args[0]` is the command name.
4. L22-27 **Pipe detection**: `os.Stdin.Stat()`; if `ModeNamedPipe`, reads all stdin and **appends** `["-pipe", string(bytes)]` to the *end* of `args` (after the subcommand). Plugins scan `args` for `-pipe` by index.
5. L29-35 **Aliases**: only `gp` → `genpwd`.
6. L37-41 `-h`/`-help`/`--help` → `fmt.Println(service.Help())`.
7. L43-55 Iterates `service.Plugin{}.List()` (which calls `Init()` on every plugin), matches `plugin.GetCommand() == firstArg`, then `plugin.Run(args[1:])`. Unknown commands **silently do nothing** (no fallback). `defer plugin.Stop()` runs per match.

**Plugin registry** (`service/plugin.go`):
- `PluginTemplate` interface L14-25 — 9 methods: `Init() error`, `Run(args []string) error`, `Stop() error`, `GetName/GetVersion/GetDescription/GetCommand/GetAuthor() string`, `GetArgs() map[string]string`.
- `PluginInfo` struct L27-35; `Plugin` struct L36; `GetInfo` L38; `Info` L49.
- `List()` L60 (value receiver) — constructs the slice, calls `Init()` on each at L71-73, returns. `Init()` is effectively the constructor.

**Registered plugins** (`service/plugin.go` import block L3-12, `List()` body L61-74) — all 8 are registered:

| Command | Plugin struct | Alias | Purpose |
|---|---|---|---|
| `v json2excel` | `Json2Excel` | — | JSON → .xlsx/CSV with dot-path key drill + flatten |
| `v jv` | `Jv` | — | JSON viewer/formatter + interactive TUI tree (default mode) |
| `v diff` | `Diff` | — | Side-by-side text diff (Myers) with inline word highlighting |
| `v fx` | `Fx` | — | Wraps external `fx` binary; pipes clipboard (URL/file/JSON) to it |
| `v genpwd` | `Genpwd` | `gp` | CSPRNG password generator with interactive TUI |
| `v pwd` | `Pwd` | — | Print cwd + copy to clipboard |
| `v tt` | `TT` | — | Unix timestamp ↔ date string conversion |
| `v tr` | `Tr` | — | Translate via Google + CNKI; Youdao word lookup |

`plugin/template/` is a **copy-paste scaffold only** — NOT imported, NOT registered.

## Key Directories

```
v/
├── main.go                 # Entry point: settings load, pipe/alias/help dispatch, plugin routing
├── service/
│   ├── plugin.go           # PluginTemplate interface (L14-25), PluginInfo, Plugin.List() registry (L60)
│   └── help.go             # Help() string builder; VVersion="0.0.5" (L9); heavy gookit/color
├── setting/
│   └── ini.go              # ~/.v_tools/settings.ini; InitSetting()/SaveSetting()/Set()
├── plugin/
│   ├── pwd/                # cwd + clipboard (simplest plugin; reference for new plugins)
│   ├── tt/                 # tt.go (Run) + implement.go (bidirectional conversion heuristic)
│   ├── json2excel/         # json2excel.go (flags) + implement.go (JSONProcessor, excelize, CSV BOM)
│   ├── translate/          # tr.go + cnki.go (AES-ECB) + youdao.go
│   ├── genpwd/             # genpwd.go (crypto/rand + Fisher-Yates) + viewer.go (TUI form)
│   ├── jv/                 # jv.go + viewer.go (~2162-line TUI) + lexer.go + filter.go + format.go + orderedmap.go + clipboard.go
│   ├── diff/               # diff.go + myers.go (Myers from scratch) + viewer.go (dual tview.TextView)
│   ├── fx/                 # fx.go (clipboard sniffer → external fx binary)
│   └── template/           # Scaffold; NOT registered
├── cmd/install.sh          # Bash installer: download latest release zip, SHA256-verify, install
├── .github/workflows/release.yml  # Release build matrix (linux/windows/darwin/android × amd64/arm64)
├── go.mod / go.sum         # Module v, Go 1.24.3
└── README.md               # User docs + command examples (license line is wrong — see above)
```

## Development Commands

### Build
```bash
go build -o v .                              # local build
go build -trimpath -ldflags "-w -s" -o v .   # release-optimized (matches CI)
```

Cross-compile (CGO disabled except Android):
```bash
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o v-darwin-arm64 .
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o v-linux-amd64 .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o v-windows-amd64.exe .
# Android REQUIRES NDK + CGO:
GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
  CC=$ANDROID_NDK_LATEST_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android32-clang \
  go build -trimpath -ldflags "-w -s" -o v-android-arm64 .
```

### Test
```bash
go test ./...                                  # full suite
go test -v ./plugin/jv/...                     # specific package (only jv has tests)
go test -run TestEvalFilter ./plugin/jv/...    # single test
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out   # coverage
go test -race ./...                            # race detector
```

### Lint / Format
```bash
go vet ./...
gofmt -w .
goimports -w .   # install: go install golang.org/x/tools/cmd/goimports@latest
```

### Module
```bash
go mod download && go mod tidy && go mod verify
```

## Code Conventions & Common Patterns

### Plugin struct layout (identical across all plugins)
```go
type Pwd struct {
    name        string
    version     string
    description string
    command     string
    args        map[string]string
    author      string
}
```
`Init()` populates all six fields; the 8 `Get*` methods are one-liners returning the field. `service.Plugin.List()` calls `Init()` on every plugin, so `Init()` is the constructor.

### Argument parsing — hand-rolled, NO `flag` package, NO cobra/urfave
Two styles coexist:
```go
// Index-based for-loop (jv, diff, genpwd):
for i := 0; i < len(args); i++ {
    switch args[i] {
    case "-l":
        if i+1 < len(args) { length = args[i+1]; i++ }
    }
}
// Range-style (json2excel):
for key, arg := range args {
    if arg == "-i" && len(args) > key+1 { inputPath = args[key+1] }
}
```
Pipe data arrives as a trailing `-pipe <data>` pair; plugins read it as a value flag.

### Error handling
- Wrap with `fmt.Errorf("...: %w", err)` (pwd, diff, genpwd, jv, fx).
- ⚠️ Inconsistent: `translate/tr.go` and `json2excel/implement.go` use `%v` instead of `%w`. Prefer `%w` for new code.
- Plugins return errors; `main.go` prints and returns. `setting.InitSetting()` **panics** on init failure (the only panic path).
- Never `_ = err`.

### Color / output
Three parallel styling mechanisms — pick by context:
- `gookit/color` — `service/help.go` (every line) and `genpwd.go`. `jv/format.go` defines reusable `color.New(...)` styles (`colorKey` FgCyan, `colorString` FgGreen, `colorNumber` FgYellow, `colorBool` FgMagenta, `colorNull` FgRed+Bold, `colorPunct` FgDarkGray, `colorIndex`/`colorBracket` FgBlue).
- Raw ANSI `\033[...m` — `diff.go` (`printInline`) and `translate/tr.go` (color consts L33-46). Older code; avoid for new TUI output.
- tview `[color]` tags + `tcell.StyleDefault.Foreground(...)` — the TUI viewers (`jv/viewer.go`, `diff/viewer.go`, `genpwd/viewer.go`).

### TUI stack
`github.com/gdamore/tcell/v2` (low-level screen/input) + `github.com/rivo/tview` (widgets). `jv/viewer.go` is a custom `Viewer` embedding `*tview.Box` with manual `Draw` (~2162 lines). `diff/viewer.go` uses dual `tview.TextView` side-by-side. `genpwd/viewer.go` uses `tview.Flex` form.

### Strings
- `strings.Builder` for all loop concatenation (`help.go`, `jv/format.go`, `diff/myers.go`, `diff/viewer.go`).
- `jv/orderedmap.go` — `OrderedMap` preserves JSON key insertion order via `json.Decoder` + `UseNumber`.

### Clipboard
`github.com/atotto/clipboard` — `clipboard.WriteAll` (pwd, genpwd, jv/clipboard.go wrapper) and `clipboard.ReadAll` (jv default source, `diff -clip`, `fx` primary input).

### Notable per-plugin internals
- **jv**: custom `lexer.go` tolerates *invalid/in-progress* JSON for live syntax highlighting; `filter.go` implements a jq-like DSL (`.key`, `["k"]`, `[0]`, `[-1]`, `.length`, `.map(.k)`); full editor with undo/redo + `$EDITOR` integration.
- **diff**: `myers.go` is a from-scratch Myers shortest-edit-script (V-array as `map[int]int`); `opChange` sentinel (`Op=99`) pairs changed lines; word-level inline diff reuses `myersDiff` over word tokens.
- **translate/cnki.go**: AES-ECB with hardcoded key `4e87183cfd3a45fe`, PKCS7 padding hand-rolled, cookie cached in `/tmp/v_cookie`. Uses deprecated `ioutil` — prefer `os`/`io` for new code.
- **json2excel/implement.go**: `JSONProcessor` with `Flatten`/`Escape`/`KeyDrill`; default output `~/Downloads/export_<unixnano>.xlsx`; CSV gets UTF-8 BOM via `golang.org/x/text/transform`.
- **genpwd**: `crypto/rand.Int` CSPRNG, guarantees ≥1 char per selected set, then Fisher-Yates shuffle; entropy = `log2(charset)*length`.
- **fx**: only plugin with NO `-pipe` handling — clipboard-only input; requires external `fx` binary (`exec.LookPath`).
- **tt/implement.go**: bidirectional heuristic — input containing `-` → date→timestamp (`time.Parse`); else timestamp→date; truncates >10-digit timestamps to 10 for millisecond compat.

### Duplicated helpers
`expandHome(path)` is copy-pasted in `jv.go` and `diff.go`. If you need it elsewhere, prefer copying the local pattern over introducing a shared util (no shared util package exists).

### Emoji in output
`📦 👤 🏠 📁 ✅ 🔑 ⚙` appear in `help.go`, `pwd.go`, `genpwd.go`, `diff.go`. Match the existing emoji style when adding user-facing output.

## Important Files

| File | Role |
|---|---|
| `main.go` | Entry point; settings init, pipe/alias/help dispatch, plugin routing (57 lines) |
| `service/plugin.go` | `PluginTemplate` interface (L14-25), `Plugin.List()` registry (L60) — **register new plugins here** |
| `service/help.go` | `Help()` colored output builder; `VVersion` constant (L9) |
| `setting/ini.go` | Config at `~/.v_tools/settings.ini`; `InitSetting()` (L11, panics on err), `SaveSetting()` (L34), `Set(section, key, value)` (L45) |
| `plugin/template/template.go` | Copy-paste scaffold for new plugins (NOT registered) |
| `.github/workflows/release.yml` | Release CI: matrix build, zip + SHA256, upload to GitHub Release |
| `cmd/install.sh` | Bash installer: GitHub API → download zip → SHA256 verify → install (sudo if needed) |
| `go.mod` | Module `v`, Go 1.24.3, 8 direct deps |

## Runtime / Tooling Preferences

- **Runtime**: Go 1.24.3 only. No Node/Bun/Python runtime involvement.
- **Package manager**: Go modules (`go mod`). Run `go mod tidy` after adding deps.
- **Config location**: `~/.v_tools/settings.ini` (user home, NOT the project dir). Created on first run by `InitSetting()`.
- **External dependency**: `v fx` requires the external `fx` binary installed separately (`brew install fx`, `go install`, or `scoop`).
- **No Makefile/Taskfile** — all build logic lives in `.github/workflows/release.yml`; use `go build` directly.
- **Target binary name** is `v` (matches `.gitignore` entry and `BINARY_PREFIX` in CI). Don't commit the `v` binary.

## Testing & QA

- **Framework**: stdlib `testing` only. Assertions via `t.Errorf` / `t.Fatalf` / `t.Helper()`. No testify (present transitively via tview, never imported). No mocks/fakes.
- **TUI tests**: `github.com/gdamore/tcell/v2` `NewSimulationScreen("UTF-8")` drives `Draw()` headlessly; assertions via `s.GetContent(x,y)` / `s.GetCursor()`.
- **Coverage reality**: ONLY `plugin/jv` has tests — 2 files, 20 funcs, white-box (`package plugin_jv`, accesses unexported fields). Zero tests in `pwd`, `tt`, `json2excel`, `translate`, `diff`, `genpwd`, `fx`, `template`, `service/`, `setting/`, and `main.go`. New behavioral contracts in those packages are currently unverified — add tests when changing load-bearing logic there.
- **Style**: flat tests, no `t.Run` subtests (except none currently). The only table-driven test is `TestEvalFilter` (`cases := []struct{expr,want string}{}`). Inline raw-string fixtures; no `testdata/`, no golden files, no build tags.
- **CI gate**: ⚠️ `.github/workflows/release.yml` builds and publishes release assets but does **NOT** run `go test` or `go vet`. There is no test/lint gate before release. Run `go test ./...` and `go vet ./...` locally before tagging a release.

## Adding a New Plugin

1. `cp -r plugin/template plugin/<name>` (or create `plugin/<name>/<name>.go`).
2. Implement all 9 `PluginTemplate` methods; populate the 6 struct fields in `Init()`.
3. Set `command` to the user-facing subcommand (e.g. `v <name>`).
4. **Register** in `service/plugin.go`: add import `plugin_<name> "v/plugin/<name>"` (underscore alias matches dir) and `&(plugin_<name>.<Struct>{})` to `List()`.
5. Add an alias in `main.go` (L29-35) only if a short form is desired.
6. `go build -o v . && ./v <name> -h` to smoke test.
7. Update `README.md` with command docs.
