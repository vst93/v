package service

import (
	"github.com/gookit/color"

	"strings"
)

// VVersion is the binary version. It stays "dev" for plain `go build` and is
// overwritten at link time by the release build:
//
//	go build -ldflags "-X v/service.VVersion=0.0.6" .
var VVersion = "dev"

func Help() string {
	pluginList := Plugin{}

	var out strings.Builder

	out.WriteString(color.New(color.FgCyan, color.Bold).Sprint("v - Gadgets under the terminal"))
	out.WriteString("\n")
	out.WriteString(color.New(color.FgGreen).Sprint("Version: "))
	out.WriteString(VVersion)
	out.WriteString("  ")
	out.WriteString(color.New(color.FgBlue).Sprint("🏠 https://github.com/vst93/v"))
	out.WriteString("\n\n")

	out.WriteString(color.New(color.FgMagenta, color.Bold).Sprint("Available Plugins"))
	out.WriteString("\n")
	out.WriteString(color.New(color.FgDarkGray).Sprint(strings.Repeat("=", 50)))
	out.WriteString("\n")

	for _, plugin := range pluginList.List() {
		info := pluginList.GetInfo(plugin)

		out.WriteString(color.New(color.FgYellow, color.Bold).Sprint("📦 " + info.Name))
		out.WriteString(" ")
		out.WriteString(color.New(color.FgGreen).Sprint(info.Version))
		out.WriteString(" ")
		out.WriteString(color.New(color.FgDarkGray).Sprint("👤 ") + color.New(color.FgBlue).Sprint(info.Author))
		if aliases := plugin.GetAliases(); len(aliases) > 0 {
			out.WriteString("  ")
			out.WriteString(color.New(color.FgMagenta).Sprint("(aliases: " + strings.Join(aliases, ", ") + ")"))
		}
		out.WriteString("\n")

		out.WriteString(color.New(color.FgWhite).Sprint("  " + info.Description))
		out.WriteString("\n\n")
	}

	out.WriteString(color.New(color.FgDarkGray).Sprint(strings.Repeat("-", 50)))
	out.WriteString("\n")
	out.WriteString(color.New(color.FgCyan).Sprint("Run "))
	out.WriteString(color.New(color.FgGreen, color.Bold).Sprint("v <command> -h"))
	out.WriteString(color.New(color.FgCyan).Sprint(" for detailed help."))
	out.WriteString("\n")

	result := strings.TrimSpace(out.String())
	return result
}
