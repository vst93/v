package plugin_tt

import (
	"fmt"
	"time"

	"github.com/gookit/color"
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
		"-h": "Show help",
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
	switch input {
	case "-m":
		// print now timesteamp in millisecond
		fmt.Println(time.Now().UnixNano() / 1e6)
		return nil
	case "-h", "-help", "--help":
		t.printHelp()
		return nil
	}
	fmt.Println(tt(input))
	return nil
}

func (t *TT) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>tt - Timestamp Converter v%s</>\n\n", t.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v tt                        Current Unix timestamp (seconds)")
	color.Println("  v tt <green>-m</>                     Current Unix timestamp (milliseconds)")
	color.Println("  v tt 1641038400             Timestamp -> date string")
	color.Println("  v tt '2022-01-01 12:00:00'  Date string -> timestamp")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-m</>   Return a millisecond timestamp")
	color.Println("  <green>-h</>   Show this help")
	color.Println()
	color.Println("<gray>Direction is inferred from the input: text containing '-' is parsed</>")
	color.Println("<gray>as a date, anything else as a timestamp.</>")
	color.Println("<gray>--------------------------------------------------</>")
}
func (t *TT) Stop() error {
	return nil
}
