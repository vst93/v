package service

func Help() string {
	pluginList := Plugin{}

	out := "Plugins:\n"
	for _, plugin := range pluginList.List() {
		out += "-------------------\n"
		out += pluginList.Info(plugin) + "\n"
	}
	return out
}
