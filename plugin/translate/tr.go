package plugin_translate

import (
	"fmt"
	"strings"
	"v/setting"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

type Tr struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (t *Tr) Init() error {
	t.name = "tr"
	t.version = "0.0.1"
	t.description = "Translate between Chinese and English (requires internet)"
	t.command = "tr"
	t.args = map[string]string{
		"-to <en|zh>":                     "Target language (default: auto-detect)",
		"-pipe":                           "Read from pipe/stdin (auto-detected)",
		"-clip":                           "Read text from clipboard",
		"-raw":                            "Print only the first successful translation, no colors",
		"-copy":                           "Copy the first successful translation to clipboard",
		"-[disable/enable]-[google/cnki]": "Disable/Enable Google Translate or CNKI Translate",
		"-h":                              "Show help",
	}
	t.author = "vst"
	return nil
}

func (t *Tr) GetName() string {
	return t.name
}
func (t *Tr) GetVersion() string {
	return t.version
}
func (t *Tr) GetDescription() string {
	return t.description
}
func (t *Tr) GetCommand() string {
	return t.command
}
func (t *Tr) GetArgs() map[string]string {
	return t.args
}
func (t *Tr) GetAuthor() string {
	return t.author
}

func (t *Tr) GetAliases() []string { return nil }

func (t *Tr) Run(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	if opts.help {
		t.printHelp()
		return nil
	}
	if opts.config != "" {
		parts := strings.Split(strings.TrimPrefix(opts.config, "-"), "-")
		if err := setting.Set("tr_"+parts[1], parts[0], "translate"); err != nil {
			return fmt.Errorf("save translation settings: %w", err)
		}
		name := map[string]string{"google": "Google", "cnki": "CNKI"}[parts[1]]
		fmt.Printf("%s Translate %sd\n", name, parts[0])
		return nil
	}
	input := opts.input
	if !opts.hasInput && opts.clip {
		input, err = clipboard.ReadAll()
		if err != nil {
			return fmt.Errorf("read clipboard: %w", err)
		}
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("no text to translate: pass text, pipe input, or use -clip (see v tr -h)")
	}
	return search(input, targetLanguage(input, opts.target), opts.raw, opts.copy)
}

type options struct {
	input    string
	hasInput bool
	target   string
	clip     bool
	raw      bool
	copy     bool
	help     bool
	config   string
}

func parseOptions(args []string) (options, error) {
	var opts options
	var words []string
	var pipe string
	var hasPipe bool
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "-h", "-help", "--help":
			opts.help = true
			return opts, nil
		case "-pipe", "-to":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			i++
			if arg == "-pipe" {
				pipe, hasPipe = args[i], true
			} else {
				opts.target = strings.ToLower(args[i])
				if opts.target != "en" && opts.target != "zh" {
					return opts, fmt.Errorf("unsupported target language %q: use -to en or -to zh", args[i])
				}
			}
		case "-clip":
			opts.clip = true
		case "-raw":
			opts.raw = true
		case "-copy":
			opts.copy = true
		case "-disable-google", "-enable-google", "-disable-cnki", "-enable-cnki":
			if opts.config != "" {
				return opts, fmt.Errorf("use one translation configuration flag at a time")
			}
			opts.config = arg
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %q (see v tr -h)", arg)
			}
			words = append(words, arg)
		}
	}
	opts.input, opts.hasInput = strings.Join(words, " "), len(words) > 0
	if hasPipe {
		opts.input, opts.hasInput = pipe, true
	}
	if opts.config != "" && (opts.hasInput || opts.clip || opts.raw || opts.copy || opts.target != "") {
		return opts, fmt.Errorf("use translation configuration flags on their own")
	}
	return opts, nil
}

func (t *Tr) Stop() error {
	return nil
}

func (t *Tr) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>tr - Text Translator v%s</>\n\n", t.version)
	color.Println("Translate between Chinese and English using Youdao, Google and CNKI (requires internet).")
	color.Println("Youdao → CNKI → Google, with independent loading indicators and inline results.")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v tr 'Hello World'           Translate text")
	color.Println("  v tr '你好'                   Translate Chinese to English")
	color.Println("  echo 'text' | v tr           Translate from pipe")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-to en|zh</>                   Set translation target (dictionary lookup auto-detects)")
	color.Println("  <green>-clip</>                       Read text from clipboard")
	color.Println("  <green>-pipe</>                       Read from pipe/stdin (auto-detected)")
	color.Println("  <green>-raw</>                        Print only the first successful translation, no colors")
	color.Println("  <green>-copy</>                       Copy the first successful translation to clipboard")
	color.Println()
	color.Println("<fg=magenta;op=bold>Configuration:</>")
	color.Println("  <green>-enable-google</>             Enable Google Translate")
	color.Println("  <green>-disable-google</>            Disable Google Translate")
	color.Println("  <green>-enable-cnki</>               Enable CNKI Translate")
	color.Println("  <green>-disable-cnki</>              Disable CNKI Translate")
	color.Println("  <green>-h</>                         Show this help")
	color.Println()
	color.Println("<fg=magenta;op=bold>Examples:</>")
	color.Println("  <gray># Translate English to Chinese</>")
	color.Println("  v tr \"Hello World\"")
	color.Println()
	color.Println("  <gray># Translate Chinese to English</>")
	color.Println("  v tr \"你好世界\"")
	color.Println()
	color.Println("  <gray># Translate mixed-language text to Chinese</>")
	color.Println("  v tr <green>-to zh</> \"Go is fast，适合 CLI\"")
	color.Println()
	color.Println("  <gray># Translate clipboard text and copy the result</>")
	color.Println("  v tr <green>-clip -copy</>")
	color.Println()
	color.Println("  <gray># Enable only Google Translate</>")
	color.Println("  v tr <green>-disable-cnki</>")
	color.Println("  v tr <green>-enable-google</>")
	color.Println()
	color.Println("<gray>Translation engines: Youdao (Chinese ↔ English) · Google Translate · CNKI</>")
	color.Println("<gray>Youdao falls back to translation when its dictionary has no entry.</>")
	color.Println("<gray>Input priority: pipe > arguments > -clip. Text containing Chinese defaults to English.</>")
	color.Println("<gray>-raw/-copy use the first successful result to arrive, including Youdao.</>")
	color.Println("<gray>Settings persist in ~/.v_tools/settings.ini under [translate] section</>")
	color.Println("<gray>Requires an active internet connection to query translation APIs</>")
	color.Println("<gray>--------------------------------------------------</>")
}
