package service

import plugin_json2excel "v/plugin/json2excel"

type PluginTemplate interface {
	Init() error
	Run(args []string) error
	Stop() error

	GetName() string
	GetVersion() string
	GetDescription() string
	GetCommand() string
	GetArgs() map[string]string
	GetAuthor() string
}

type Plugin struct{}

func (p *Plugin) Info(pl PluginTemplate) string {
	out := pl.GetName() + " " + pl.GetVersion() + " by " + pl.GetAuthor() + "\n" + pl.GetDescription() + "\n"
	out += "Command: \n  " + pl.GetCommand() + "\n"
	out += "Args:\n"
	for k, v := range pl.GetArgs() {
		out += "  " + k + ": " + v + "\n"
	}
	return out
}

func (p Plugin) List() []PluginTemplate {
	list := []PluginTemplate{
		&(plugin_json2excel.Json2Excel{}),
	}
	for _, v := range list {
		v.Init()
	}
	return list
}
