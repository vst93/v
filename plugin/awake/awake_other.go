//go:build !windows && !darwin && !linux

package plugin_awake

import "fmt"

func startPreventSleep() (func() error, error) {
	return nil, fmt.Errorf("the awake plugin does not support this operating system")
}
