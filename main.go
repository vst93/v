package main

import (
	"fmt"
	"io"
	"os"
	"v/service"
	"v/setting"
)

func main() {
	var err error

	// load settings
	setting.InitSetting()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"-h"}
	}
	firstArg := args[0]
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
		bytes, _ := io.ReadAll(os.Stdin)
		args = append(args, "-pipe")
		args = append(args, string(bytes))
	}

	// Command aliases (short forms)
	aliases := map[string]string{
		"gp":  "genpwd",
		"gc":  "gencm",
		"cc":  "codec",
		"enc": "codec", // former name, kept so existing habits keep working
	}
	if real, ok := aliases[firstArg]; ok {
		firstArg = real
	}

	switch firstArg {
	case "-help", "--help", "-h":
		fmt.Println(service.Help())
		return
	case "-v", "-version", "--version":
		fmt.Println(service.VVersion)
		return
	}

	plugins := service.Plugin{}

	for _, plugin := range plugins.List() {
		if plugin.GetCommand() == firstArg {
			err = plugin.Run(args[1:])
			if err != nil {
				fmt.Println(err)
				return
			}
			defer plugin.Stop()
			return
		}
	}

	fmt.Printf("Unknown command: %s\nRun `v -h` to see the available commands.\n", firstArg)
}
