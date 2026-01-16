package service

import (
	"github.com/gookit/color"

	"strings"
)

var VVersion = "0.0.4"

func Help() string {
	pluginList := Plugin{}

	var out strings.Builder

	out.WriteString(color.New(color.FgCyan, color.Bold).Sprint("v - Gadgets under the terminal"))
	out.WriteString("\n\n")

	out.WriteString(color.New(color.FgGreen).Sprint("Version: "))
	out.WriteString(VVersion)
	out.WriteString("\n")

	out.WriteString(color.New(color.FgBlue).Sprint("Homepage: https://github.com/vst93/v"))
	out.WriteString("\n\n")

	out.WriteString(color.New(color.FgMagenta, color.Bold).Sprint("Available Plugins"))
	out.WriteString("\n")

	out.WriteString(color.New(color.FgBlack).Sprint(strings.Repeat("=", 50)))
	out.WriteString("\n\n")

	for _, plugin := range pluginList.List() {
		info := pluginList.GetInfo(plugin)

		out.WriteString(color.New(color.FgYellow, color.Bold).Sprint(info.Name))
		out.WriteString(" ")
		out.WriteString(color.New(color.FgGreen).Sprint(info.Version))
		out.WriteString(" ")

		out.WriteString(color.New(color.FgBlue).Sprint("by "))
		out.WriteString(info.Author)
		out.WriteString("\n\n")

		out.WriteString(color.New(color.FgWhite).Sprint(info.Description))
		out.WriteString("\n\n")

		out.WriteString(color.New(color.FgCyan).Sprint("Command:"))
		out.WriteString("\n")
		out.WriteString(color.New(color.FgGreen).Sprint("  v "))
		out.WriteString(info.Command)
		out.WriteString("\n\n")

		if len(info.Args) > 0 {
			out.WriteString(color.New(color.FgCyan).Sprint("Args:"))
			out.WriteString("\n")
			for k, v := range info.Args {
				out.WriteString(color.New(color.FgMagenta).Sprint("  "))
				out.WriteString(color.New(color.FgMagenta).Sprint(k))
				out.WriteString(color.New(color.FgMagenta).Sprint(": "))
				out.WriteString(color.New(color.FgWhite).Sprint(v))
				out.WriteString("\n")
			}
			out.WriteString("\n")
		}

		out.WriteString(color.New(color.FgBlack).Sprint(strings.Repeat("-", 50)))
		out.WriteString("\n\n")
	}

	result := strings.TrimSpace(out.String())
	return result
}
