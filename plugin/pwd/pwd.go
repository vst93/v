package plugin_pwd

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
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
	p.args = map[string]string{
		"-h": "Show help",
	}
	p.author = "vst"
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
	for _, arg := range args {
		switch arg {
		case "-h", "-help", "--help":
			p.printHelp()
			return nil
		}
	}

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

func (p *Pwd) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>pwd - Print Working Directory v%s</>\n\n", p.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v pwd    Print the current directory and copy it to clipboard")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-h</>   Show this help")
	color.Println("<gray>--------------------------------------------------</>")
}
func (p *Pwd) Stop() error {
	return nil
}
