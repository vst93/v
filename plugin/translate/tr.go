package plugin_translate

import (
	"fmt"
	"strings"
	"v/setting"

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
	t.description = "Translate text (need internet connection)"
	t.command = "tr"
	t.args = map[string]string{
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
	text := strings.Join(args, " ")
	text = strings.TrimSpace(text)
	switch text {
	case "-disable-google":
		err := setting.Set("tr_google", "disable", "translate")
		if err != nil {
			return err
		}
		fmt.Println("Google Translate disabled")
		return nil
	case "-enable-google":
		err := setting.Set("tr_google", "enable", "translate")
		if err != nil {
			return err
		}
		fmt.Println("Google Translate enabled")
		return nil
	case "-disable-cnki":
		err := setting.Set("tr_cnki", "disable", "translate")
		if err != nil {
			return err
		}
		fmt.Println("CNKI Translate disabled")
		return nil
	case "-enable-cnki":
		err := setting.Set("tr_cnki", "enable", "translate")
		if err != nil {
			return err
		}
		fmt.Println("CNKI Translate enabled")
		return nil
	case "-h", "-help", "--help":
		t.printHelp()
		return nil
	default:
	}
	if text == "" {
		fmt.Println("Please input text to translate")
		return nil
	}
	Search(text)

	return nil
}
func (t *Tr) Stop() error {
	return nil
}

func (t *Tr) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>tr - Text Translator v%s</>\n\n", t.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v tr 'Hello World'    Translate text (Google + CNKI, Youdao for words)")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-enable-google</>    Enable Google Translate")
	color.Println("  <green>-disable-google</>   Disable Google Translate")
	color.Println("  <green>-enable-cnki</>      Enable CNKI Translate")
	color.Println("  <green>-disable-cnki</>     Disable CNKI Translate")
	color.Println("  <green>-h</>                Show this help")
	color.Println()
	color.Println("<gray>Requires an internet connection. Settings persist in ~/.v_tools/settings.ini.</>")
	color.Println("<gray>--------------------------------------------------</>")
}
