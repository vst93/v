package service

import "strings"

var VVersion = "0.0.3"

func Help() string {
	pluginList := Plugin{}

	out := "v - Gadgets under the terminal\nVersion: " + VVersion + "\nHomepage: https://github.com/vst93/v" + "\n\n"
	out += "Plugins:\n"
	for _, plugin := range pluginList.List() {
		out += "-------------------\n"
		out += pluginList.Info(plugin) + "\n"
	}
	out = strings.TrimSpace(out)
	return out
}
