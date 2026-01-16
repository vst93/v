package plugin_translate

import (
	"fmt"
	"strings"
	"v/setting"
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
		"-[disable/enable]-[google/cnki]": "Disable/Enable Google Translate or Bing Translate",
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
		fmt.Println("Bing Translate disabled")
		return nil
	case "-enable-cnki":
		err := setting.Set("tr_cnki", "enable", "translate")
		if err != nil {
			return err
		}
		fmt.Println("Bing Translate enabled")
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
