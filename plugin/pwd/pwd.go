package plugin_pwd

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
)

type Pwd struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (p *Pwd) Init() error {
	p.name = "pwd"
	p.version = "0.0.1"
	p.description = "print working directory and copy to clipboard"
	p.command = "pwd"
	p.args = map[string]string{}
	p.author = "v"
	return nil
}

func (p *Pwd) GetName() string {
	return p.name
}
func (p *Pwd) GetVersion() string {
	return p.version
}
func (p *Pwd) GetDescription() string {
	return p.description
}
func (p *Pwd) GetCommand() string {
	return p.command
}
func (p *Pwd) GetArgs() map[string]string {
	return p.args
}
func (p *Pwd) GetAuthor() string {
	return p.author
}

func (p *Pwd) Run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	fmt.Printf("📁 Current Directory:\n\n%s\n", cwd)

	if err := clipboard.WriteAll(cwd); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	fmt.Println("\n✅ Copied to clipboard")
	return nil
}
func (p *Pwd) Stop() error {
	return nil
}
