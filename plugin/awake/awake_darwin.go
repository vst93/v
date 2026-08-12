//go:build darwin

package plugin_awake

import (
	"fmt"
	"os/exec"
)

func startPreventSleep() (func() error, error) {
	path, err := exec.LookPath("caffeinate")
	if err != nil {
		return nil, fmt.Errorf("caffeinate is required to prevent sleep: %w", err)
	}
	cmd := exec.Command(path, "-dimsu")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start caffeinate: %w", err)
	}
	return func() error {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop caffeinate: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			// Killing caffeinate is the normal cleanup path.
			if _, ok := err.(*exec.ExitError); ok {
				return nil
			}
			return err
		}
		return nil
	}, nil
}
