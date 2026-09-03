# Repository Guidelines

## Project Overview

**v** is a single-binary, plugin-based command-line utility ("gadgets under the terminal") built in Go. It bundles developer tools — timestamp conversion, JSON viewer/formatter, JSON→Excel export, text diff, password generation, translation, clipboard helpers — behind one `v <command>` dispatcher. Cross-compiled for macOS, Linux, Windows, and Android.

- **Module**: `v` (repo root is the main package)
- **Go version**: 1.24.3 (no `toolchain` directive)
- **Repo**: https://github.com/vst93/v
- **License**: GNU GPL v3 — see `LICENSE` (and the matching line at the bottom of `README.md`).

## Architecture & Data Flow

```
stdin/pipe ──► main.go ──► service.Plugin{}.List() ──► matched plugin.Run(args[1:]) ──► stdout / clipboard / TUI
                  │
                  ├─ setting.InitSetting()  (~/.v_tools/settings.ini)
                  ├─ pipe detection          (appends `-pipe <data>` to args)
                  ├─ aliases (from plugins' GetAliases): gp, gc, cc, j2e, s2f
                  ├─ -h/-help/--help ──► service.Help()
                  └─ -v/-version/--version ──► service.VVersion
```

**Dispatch flow** (`main.go`, `main()` at L11):
1. L15 `setting.InitSetting()` loads `~/.v_tools/settings.ini`.
2. L17-20 `args := os.Args[1:]`; if empty, defaults to `["-h"]`.
3. L21 `firstArg := args[0]` is the command name.
4. L22-27 **Pipe detection**: `os.Stdin.Stat()`; if `ModeNamedPipe`, reads all stdin and **appends** `["-pipe", string(bytes)]` to the *end* of `args` (after the subcommand). Plugins scan `args` for `-pipe` by index.
5. **Aliases**: built from each plugin's `GetAliases()`; currently `gp` → `genpwd`, `gc` → `gencm`, `cc` → `codec`, `j2e` → `json2excel`, `s2f` → `save2file`.
6. `-h`/`-help`/`--help` → `service.Help()`; `-v`/`-version`/`--version` → `service.VVersion`.
7. Iterates `service.Plugin{}.List()` (which calls `Init()` on every plugin), matches `plugin.GetCommand() == firstArg`, then `plugin.Run(args[1:])`. Unknown commands print a hint pointing at `v -h`. `defer plugin.Stop()` runs per match.

**Plugin registry** (`service/plugin.go`):
- `PluginTemplate` interface L14-25 — 9 methods: `Init() error`, `Run(args []string) error`, `Stop() error`, `GetName/GetVersion/GetDescription/GetCommand/GetAuthor() string`, `GetArgs() map[string]string`.
- `PluginInfo` struct L27-35; `Plugin` struct L36; `GetInfo` L38; `Info` L49.
- `List()` L60 (value receiver) — constructs the slice, calls `Init()` on each at L71-73, returns. `Init()` is effectively the constructor.

**Registered plugins** (`service/plugin.go` import block, `List()` body) - all 13 are registered:

| Command | Plugin struct | Alias | Purpose |
|---|---|---|---|
| `v json2excel` | `Json2Excel` | `j2e` | JSON → .xlsx/CSV with dot-path key drill + flatten |
| `v jv` | `Jv` | — | JSON viewer/formatter + interactive TUI tree (default mode) |
| `v diff` | `Diff` | — | Side-by-side text diff (Myers) with inline word highlighting |
| `v codec` | `Codec` | `cc` | Encode/decode base64, base32, url, hex, html, unicode + TUI |
| `v cp` | `Cp` | — | Copy text to clipboard (pipe-friendly, with trim options) |
| `v gencm` | `Gcm` | `gc` | AI commit message from git diff via OpenAI-compatible API |
| `v genpwd` | `Genpwd` | `gp` | CSPRNG password generator with interactive TUI |
| `v pwd` | `Pwd` | — | Print cwd + copy to clipboard |
| `v save2file` | `Save` | `s2f` | Save clipboard/pipe/arg text to a timestamped file and reveal it |
| `v tt` | `TT` | — | Unix timestamp ↔ date string conversion |
| `v tr` | `Tr` | — | Translate via Google + CNKI; Youdao word lookup |
| `v vc` | `Vc` | — | v-connection: multi-device text sharing via a channel |
| `v awake` | `Awake` | — | Keep the system awake with a live status display |

`plugin/template/` is a **copy-paste scaffold only** — NOT imported, NOT registered.

## Key Directories

```
v/
├── main.go                 # Entry point: settings load, pipe/alias/help/version dispatch, plugin routing
├── Makefile                # build / test / release targets; owns the version-injecting ldflags
├── service/
│   ├── plugin.go           # PluginTemplate interface, PluginInfo, Plugin.List() registry
│   └── help.go             # Help() string builder; VVersion ("dev", injected at link time)
├── setting/
│   └── ini.go              # ~/.v_tools/settings.ini; InitSetting()/SaveSetting()/Set()
├── plugin/
│   ├── pwd/                # cwd + clipboard (simplest plugin; reference for new plugins)
│   ├── tt/                 # tt.go (Run) + implement.go (bidirectional conversion heuristic)
│   ├── json2excel/         # json2excel.go (flags) + implement.go (JSONProcessor, excelize, CSV BOM)
│   ├── translate/          # tr.go + cnki.go (AES-ECB) + youdao.go
│   ├── genpwd/             # genpwd.go (crypto/rand + Fisher-Yates) + viewer.go (TUI form)
│   ├── jv/                 # jv.go + viewer.go (~2585-line TUI) + lexer.go + filter.go + format.go + orderedmap.go + clipboard.go
│   ├── internal/theme/     # shared TUI theme: light/dark detection (OSC 11, COLORFGBG, V_THEME) + semantic palette
│   ├── diff/               # diff.go + myers.go (Myers from scratch) + viewer.go + paste.go
│   ├── cp/                 # cp.go — copy to clipboard, pipe-first
│   ├── codec/              # codec.go (12 codec modes) + viewer.go (tview List/TextArea TUI)
│   ├── gcm/                # gcm.go (git diff → OpenAI-compatible chat/completions) + gcm_test.go
│   └── template/           # Scaffold; NOT registered
├── cmd/install.sh          # Bash installer: resolve release, download zip, SHA256-verify, install
├── .github/workflows/release.yml  # test gate + release build matrix (linux/windows/darwin/android × amd64/arm64)
├── go.mod / go.sum         # Module v, Go 1.24.3
└── README.md               # User docs + command examples
```

## Development Commands

### Build

The software version is injected at link time. `service.VVersion` defaults to
`"dev"`, so a plain `go build` reports `dev` (development mode); release builds
overwrite it with the release tag, minus any leading `v`.

```bash
make build                       # dev build (VERSION=dev)
make build VERSION=0.0.6         # inject a specific version
make test                        # go vet ./... && go test ./...
make release VERSION=0.0.6       # cross-compile all 7 platforms into ./output
make help                        # list targets

go build -o v .                  # equivalent to `make dev`; reports "dev"
```

⚠️ `go build -o v main.go` does **not** work — the root package spans several
files, so the target must be `.`.

The ldflags used by both the Makefile and CI:
```
-w -s -X v/service.VVersion=$(VERSION)
```

Cross-compile by hand (CGO disabled except Android):
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

**Reserved cross-plugin flags. These names mean exactly the same thing in every
plugin. A plugin-specific flag must never reuse one of them.**

| Flag | Meaning |
|---|---|
| `-pipe` | pipe/stdin input (`main.go` appends `-pipe <data>` automatically when stdin is a pipe) |
| `-file <path>` | read input from a file (`~` expanded via the local `expandHome`) |
| `-clip` | **read** the clipboard as input |
| `-url <url>` | read input from a URL |
| `-out <path>` | write the result to a file |
| `-copy` | **write** the result to the clipboard |
| `-tui` | interactive TUI mode |
| `-raw` | plain text output, no colors |
| `-h` | help (always accept `-h`, `-help` and `--help` together) |

Note the deliberate `-clip` (read) vs `-copy` (write) split — the old `-c`
meant "copy to clipboard" in some plugins and "compress"/"content" in others,
which is exactly what this convention exists to prevent.

**Input source priority, uniform across plugins**:
`-pipe` > `-file` > `-url` > positional argument > `-clip`/clipboard default.

**Plugin-specific flags** use multi-letter words (`-sort` `-inline` `-unexpand`
`-trim` `-add` `-b64`). Single letters are free for a plugin's own mode
switches, since every reserved name above is a whole word and cannot collide.

**Three intentional exceptions** — do not "fix" these:
- `jv` keeps its established mode letters `-f` (format) `-c` (compress) `-e`
  (escape) `-u` (unescape) `-i` (interactive). `-tui` is accepted as a synonym
  for `-i`.
- `diff` takes two inputs, so it uses `-left`/`-right` instead of `-file`.
- `codec`'s `-url`/`-urld` are **codec names** (URL percent-encoding), not the
  URL input source. `codec` has no URL-fetching feature, so there is no real
  clash.
- `json2excel` still accepts `-i`/`-o`/`-c` as undocumented back-compat aliases
  for `-file`/`-out`/positional content.

Parsing style — index-based `for` loop with a `switch`, which is what every
plugin except the oldest ones uses:
```go
for i := 0; i < len(args); i++ {
    switch args[i] {
    case "-file":
        if i+1 < len(args) { filePath = args[i+1]; i++ }
    case "-copy":
        toClip = true
    case "-h", "-help", "--help":
        p.printHelp()
        return nil
    default:
        // positional argument = literal input text
        if !strings.HasPrefix(args[i], "-") && !hasInput {
            input = args[i]; hasInput = true
        }
    }
}
```
Pipe data arrives as a trailing `-pipe <data>` pair; plugins read it as a value flag.

### Error handling
- Wrap with `fmt.Errorf("...: %w", err)` (pwd, diff, genpwd, jv).
- ⚠️ Inconsistent: `translate/tr.go` and `json2excel/implement.go` use `%v` instead of `%w`. Prefer `%w` for new code.
- Plugins return errors; `main.go` prints and returns. `setting.InitSetting()` **panics** on init failure (the only panic path).
- Never `_ = err`.

### Color / output
Three parallel styling mechanisms — pick by context:
- `gookit/color` - `service/help.go` (main help) and every plugin's `printHelp()` (tag-based: `<fg=cyan;op=bold>` title, `<fg=magenta;op=bold>` section headers, `<green>` flags, `<gray>` I/O notes). `genpwd.go` also uses `color.Green.Sprint` for output. `jv/format.go` defines reusable `color.New(...)` styles (`colorKey` FgCyan, `colorString` FgGreen, `colorNumber` FgYellow, `colorBool` FgMagenta, `colorNull` FgRed+Bold, `colorPunct` FgDarkGray, `colorIndex`/`colorBracket` FgBlue).
- Raw ANSI `\033[...m` — `diff.go` (`printInline`) and `translate/tr.go` (color consts L33-46). Older code; avoid for new TUI output.
- tview `[color]` tags + `tcell.StyleDefault.Foreground(...)` — the TUI viewers (`jv/viewer.go`, `diff/viewer.go`, `genpwd/viewer.go`). All four TUI plugins resolve colors through `internal/theme` (see "Terminal theme standard" below) instead of hardcoding them.

### TUI stack
`github.com/gdamore/tcell/v2` (low-level screen/input) + `github.com/rivo/tview` (widgets). **Every TUI must call `EnableMouse(true)`** on the application — mouse and touch input both arrive as mouse events, so skipping it silently disables both. `jv/viewer.go` is a custom `Viewer` embedding `*tview.Box` with manual `Draw` and a hand-written `MouseHandler` (~2585 lines). `diff/viewer.go` uses dual `tview.TextView` side-by-side. `genpwd/viewer.go` uses a `tview.Flex` form. `codec/viewer.go` composes stock widgets (`tview.List` for the selectors, `tview.TextArea` for input, `tview.Button` for actions) precisely so that click/tap/drag/wheel come from tview rather than being reimplemented — prefer this approach for new TUIs. `TextArea.SetClipboard(...)` must be wired to `atotto/clipboard` or copy/paste stays local to the widget.

### Terminal theme standard (`internal/theme`)
All TUI plugins must adapt to light and dark terminals via `internal/theme`:
1. At TUI startup (before creating any widget): `theme.Init(true)` (enables the live OSC 11 background probe on unix; falls back to `COLORFGBG`, then dark), then `theme.ApplyTView()` for tview-based plugins so stock widgets stop assuming a black terminal.
2. Take colors from `theme.Current()` / `theme.For(light)` semantic slots (`Text`, `TextDim`, `Accent`/`AccentFg`, `Success`/`Warn`/`Error`/`Info`, `Border`, `FieldBg`/`FieldFg`, `SelBg`/`SelFg`, `PanelBg`, `CursorLineBg`, `MatchBg`…). For tview dynamic tags use `"[" + theme.Hex(c) + "]"`.
3. Never hardcode theme-sensitive colors (black/white backgrounds, near-white text). Colors that carry the same meaning in both themes (red error, yellow match highlight) may stay literal.
4. Manual-draw viewers (like `jv`) map palette slots onto local style vars in one `applyTheme(light bool)` function.
5. Detection: `V_THEME=light|dark` (alias `JV_THEME`) > OSC 11 probe (only when `Init(true)`) > `COLORFGBG` > dark. Tests never touch the terminal — they pass `false` or call `theme.For(light)` directly.

### Strings
- `strings.Builder` for all loop concatenation (`help.go`, `jv/format.go`, `diff/myers.go`, `diff/viewer.go`).
- `jv/orderedmap.go` — `OrderedMap` preserves JSON key insertion order via `json.Decoder` + `UseNumber`.

### Clipboard
`github.com/atotto/clipboard` - `clipboard.WriteAll` (pwd, genpwd, jv/clipboard.go wrapper) and `clipboard.ReadAll` (jv default source, `diff -clip`).

### Notable per-plugin internals
- **jv**: custom `lexer.go` tolerates *invalid/in-progress* JSON for live syntax highlighting; `filter.go` implements a jq-like DSL (`.key`, `["k"]`, `[0]`, `[-1]`, `.length`, `.map(.k)`); full editor with undo/redo + `$EDITOR` integration; edit-mode text selection (Shift+arrows, Ctrl-A/C/X/V, word nav, mouse drag); `-url` fetches JSON via `net/http` (30s timeout).
- **diff**: `myers.go` is a from-scratch Myers shortest-edit-script (V-array as `map[int]int`); `opChange` sentinel (`Op=99`) pairs changed lines; word-level inline diff reuses `myersDiff` over word tokens.
- **translate/cnki.go**: AES-ECB with hardcoded key `4e87183cfd3a45fe`, PKCS7 padding hand-rolled, cookie cached in `/tmp/v_cookie`. Uses deprecated `ioutil` — prefer `os`/`io` for new code.
- **json2excel/implement.go**: `JSONProcessor` with `Flatten`/`Escape`/`KeyDrill`; default output `~/Downloads/export_<unixnano>.xlsx`; CSV gets UTF-8 BOM via `golang.org/x/text/transform`.
- **genpwd**: `crypto/rand.Int` CSPRNG, guarantees ≥1 char per selected set, then Fisher-Yates shuffle; entropy = `log2(charset)*length`.
- **codec**: `doTransform(mode, input)` in `codec.go` is the single source of truth for all 12 modes and is shared by the CLI and the TUI — add new codecs there, plus an entry in `codecs` in `viewer.go`. Unicode escaping handles astral code points via surrogate pairs.
- **tt/implement.go**: bidirectional heuristic - input containing `-` → date→timestamp (`time.Parse`); else timestamp→date; truncates >10-digit timestamps to 10 for millisecond compat.

### Duplicated helpers
`expandHome(path)` is copy-pasted in `jv.go`, `diff.go`, `codec.go` and `json2excel.go`. If you need it elsewhere, prefer copying the local pattern over introducing a shared util (no shared util package exists).

### Emoji in output
`📦 👤 🏠 📁 ✅ 🔑 ⚙` appear in `help.go`, `pwd.go`, `genpwd.go`, `diff.go`, `codec.go`. Match the existing emoji style when adding user-facing output.

## Important Files

| File | Role |
|---|---|
| `main.go` | Entry point; settings init, pipe/alias/help/version dispatch, plugin routing |
| `Makefile` | Build/test/release targets; the single source of the version-injecting ldflags |
| `service/plugin.go` | `PluginTemplate` interface, `Plugin.List()` registry — **register new plugins here** |
| `service/help.go` | `Help()` colored output builder (compact: name + description, no arg dump; footer points to `v <cmd> -h`); `VVersion` var |
| `setting/ini.go` | Config at `~/.v_tools/settings.ini`; `InitSetting()` (panics on err), `SaveSetting()`, `Set(section, key, value)` |
| `plugin/template/template.go` | Copy-paste scaffold for new plugins (NOT registered) |
| `.github/workflows/release.yml` | `test` job (vet + test) gates the `build` matrix; zip + SHA256, upload to GitHub Release |
| `cmd/install.sh` | Bash installer: GitHub API → download zip → SHA256 verify → install (sudo if needed) |
| `go.mod` | Module `v`, Go 1.24.3, 8 direct deps |

## Runtime / Tooling Preferences

- **Runtime**: Go 1.24.3 only. No Node/Bun/Python runtime involvement.
- **Package manager**: Go modules (`go mod`). Run `go mod tidy` after adding deps.
- **Config location**: `~/.v_tools/settings.ini` (user home, NOT the project dir). Created on first run by `InitSetting()`.
- **Build entry point**: the `Makefile`. It owns the version-injecting ldflags, and CI mirrors the same flags — change them in both places or neither.
- **Target binary name** is `v` (matches `.gitignore` entry and `BINARY_PREFIX` in CI). Don't commit the `v` binary.

## Testing & QA

- **Framework**: stdlib `testing` only. Assertions via `t.Errorf` / `t.Fatalf` / `t.Helper()`. No testify (present transitively via tview, never imported). No mocks/fakes.
- **TUI tests**: `github.com/gdamore/tcell/v2` `NewSimulationScreen("UTF-8")` drives `Draw()` headlessly; assertions via `s.GetContent(x,y)` / `s.GetCursor()`.
- **Coverage reality**: `plugin/jv` (white-box, `package plugin_jv`, accesses unexported fields), `plugin/diff` and `plugin/gcm` have tests; `plugin/codec` covers the transform table and TUI layout. Zero tests in `pwd`, `tt`, `json2excel`, `translate`, `genpwd`, `template`, `service/`, `setting/`, and `main.go`. New behavioral contracts in those packages are currently unverified - add tests when changing load-bearing logic there.
- **Style**: flat tests, no `t.Run` subtests (except none currently). The only table-driven test is `TestEvalFilter` (`cases := []struct{expr,want string}{}`). Inline raw-string fixtures; no `testdata/`, no golden files, no build tags.
- **CI gate**: `.github/workflows/release.yml` runs a `test` job (`go vet ./...` + `go test ./...`) that the `build` matrix depends on, so a failing test blocks the release assets. Still worth running `make test` locally before tagging.

## Release Process

1. `git checkout main && git pull origin main` — ensure local main is up to date.
2. Check existing tags: `git tag --sort=-v:refname | head -5` — last tag is the current version.
3. Increment patch: `0.0.5` → `0.0.6` (tag has **no** `v` prefix).
4. Gather commits since last tag: `git log <last_tag>..origin/main --oneline --no-merges`.
5. Create release with title and notes:
   ```bash
   gh release create <version> --target main --title "<version>" --notes "<notes>"
   ```
   - **Title** must be the version string itself (e.g. `0.0.6`), NOT empty — empty title makes GitHub auto-fill with commit message.
   - **Release notes**: group commits into sections (New Plugins / Improvements / Changes), written in Chinese. Don't leave notes empty.
6. `.github/workflows/release.yml` auto-triggers on `release: created` — runs the `test` gate, then builds 7 binaries (linux/windows/darwin/android × amd64/arm64, excluding windows/arm64) with `-X v/service.VVersion=<tag minus leading v>`, zips + SHA256, uploads to the release.
7. `gh run watch <run_id>` to monitor build status.
8. Verify: `gh release view <version>` — check title, notes, and assets (16 files = 7 zips + 7 sha256 + 2 source). Download one binary and confirm `v -version` prints the tag rather than `dev`.

**Release notes format** (Chinese, grouped):
```
## 新增插件

- **插件名**: 一句话描述

## 改进

- 插件名: 具体改进点

## 变更

- 移除/变更的内容
```

## Adding a New Plugin

1. `cp -r plugin/template plugin/<name>` (or create `plugin/<name>/<name>.go`).
2. Implement all 9 `PluginTemplate` methods; populate the 6 struct fields in `Init()`.
3. Set `command` to the user-facing subcommand (e.g. `v <name>`).
4. **Follow the reserved flag names above.** Every plugin must handle `-h`/`-help`/`--help`, and any input/output flag it exposes must use the reserved name for that meaning.
5. **Register** in `service/plugin.go`: add import `plugin_<name> "v/plugin/<name>"` (underscore alias matches dir) and `&(plugin_<name>.<Struct>{})` to `List()`.
6. Add an alias in `main.go`'s alias map only if a short form is desired.
7. `make build && ./v <name> -h` to smoke test.
8. Update `README.md` with command docs.
