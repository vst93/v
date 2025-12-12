package main

import (
	"fmt"
	"io"
	"os"
	"v/service"
)


func main() {
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

	switch firstArg {
	case "-help":
	case "--help":
	case "-h":
		fmt.Println(service.Help())
		return
	}

	plugins := service.Plugin{}
	var err error

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
}
