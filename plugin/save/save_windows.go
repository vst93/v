//go:build windows

package plugin_save

import (
	"os"
	"os/exec"
	"path/filepath"
)

func defaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

func revealFile(path string) error {
	return exec.Command("explorer", "/select,"+path).Run()
}
