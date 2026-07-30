package service

import (
	plugin_codec "v/plugin/codec"
	plugin_cp "v/plugin/cp"
	plugin_diff "v/plugin/diff"
	plugin_gcm "v/plugin/gcm"
	plugin_genpwd "v/plugin/genpwd"
	plugin_json2excel "v/plugin/json2excel"
	plugin_jv "v/plugin/jv"
	plugin_pwd "v/plugin/pwd"
	plugin_translate "v/plugin/translate"
	plugin_tt "v/plugin/tt"
)

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

type PluginInfo struct {
	Name        string
	Version     string
	Author      string
	Description string
	Command     string
	Args        map[string]string
}

type Plugin struct{}

func (p *Plugin) GetInfo(pl PluginTemplate) PluginInfo {
	return PluginInfo{
		Name:        pl.GetName(),
		Version:     pl.GetVersion(),
		Author:      pl.GetAuthor(),
		Description: pl.GetDescription(),
		Command:     pl.GetCommand(),
		Args:        pl.GetArgs(),
	}
}

func (p *Plugin) Info(pl PluginTemplate) string {
	info := p.GetInfo(pl)
	out := info.Name + " " + info.Version + " by " + info.Author + "\n" + info.Description + "\n"
	out += "Command: \n  " + info.Command + "\n"
	out += "Args:\n"
	for k, v := range info.Args {
		out += "  " + k + ": " + v + "\n"
	}
	return out
}

func (p Plugin) List() []PluginTemplate {
	list := []PluginTemplate{
		&(plugin_json2excel.Json2Excel{}),
		&(plugin_jv.Jv{}),
		&(plugin_diff.Diff{}),
		&(plugin_codec.Codec{}),
		&(plugin_cp.Cp{}),
		&(plugin_gcm.Gcm{}),
		&(plugin_genpwd.Genpwd{}),
		&(plugin_pwd.Pwd{}),
		&(plugin_tt.TT{}),
		&(plugin_translate.Tr{}),
	}
	for _, v := range list {
		v.Init()
	}
	return list
}
