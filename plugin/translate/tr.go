package plugin_translate

import (
	"fmt"
	"strings"
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
	t.description = "Translate text"
	t.command = "tr"
	t.args = map[string]string{}
	t.author = ""
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
