//go:build !darwin && !linux && !windows

package plugin_save

import (
	"fmt"
	"os"
)

func defaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func revealFile(path string) error {
	return fmt.Errorf("revealing files is not supported on this platform")
}
