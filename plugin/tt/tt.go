package plugin_tt

import (
	"fmt"
	"time"
)

type TT struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (t *TT) Init() error {
	t.name = "tt"
	t.version = "0.0.1"
	t.description = "provides mutual conversion of timestamp and date"
	t.command = "tt"
	t.args = map[string]string{
		"-m": "return millisecond timestamp",
	}
	t.author = "vst"
	return nil
}

func (t *TT) GetName() string {
	return t.name
}
func (t *TT) GetVersion() string {
	return t.version
}
func (t *TT) GetDescription() string {
	return t.description
}
func (t *TT) GetCommand() string {
	return t.command
}
func (t *TT) GetArgs() map[string]string {
	return t.args
}
func (t *TT) GetAuthor() string {
	return t.author
}

func (t *TT) Run(args []string) error {
	if len(args) == 0 {
		// print now timesteamp
		fmt.Println(time.Now().Unix())
		return nil
	}
	input := args[0]
	if input == "-m" {
		// print now timesteamp in millisecond
		fmt.Println(time.Now().UnixNano() / 1e6)
		return nil
	}
	// fmt.Println(input)
	fmt.Println(tt(input))
	return nil
}
func (t *TT) Stop() error {
	return nil
}
