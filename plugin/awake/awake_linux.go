//go:build linux

package plugin_awake

import (
	"fmt"
	"os/exec"
)

func startPreventSleep() (func() error, error) {
	path, err := exec.LookPath("systemd-inhibit")
	if err != nil {
		return nil, fmt.Errorf("systemd-inhibit is required to prevent sleep on Linux: %w", err)
	}
	cmd := exec.Command(path, "--what=idle:sleep", "--mode=block", "--why=v awake", "sleep", "infinity")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start systemd-inhibit: %w", err)
	}
	return func() error {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop systemd-inhibit: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return nil
			}
			return err
		}
		return nil
	}, nil
}
