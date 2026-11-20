package plugin_qr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func defaultDownloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate Downloads directory: %w", err)
	}
	if runtime.GOOS == "linux" {
		config := os.Getenv("XDG_CONFIG_HOME")
		if config == "" || !filepath.IsAbs(config) {
			config = filepath.Join(home, ".config")
		}
		if dir := xdgDownloadsDir(home, config); dir != "" {
			return dir, nil
		}
	}
	if runtime.GOOS == "android" {
		// Termux exposes Android's Downloads folder after termux-setup-storage.
		dir := filepath.Join(home, "storage", "downloads")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return filepath.Join(home, "Downloads"), nil
}

func xdgDownloadsDir(home, config string) string {
	data, err := os.ReadFile(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(key) != "XDG_DOWNLOAD_DIR" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")
		value = strings.ReplaceAll(value, "${HOME}", home)
		value = strings.ReplaceAll(value, "$HOME", home)
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
	}
	return ""
}

func openImage(path string) error {
	// Pass paths as arguments without a shell, including spaces and metacharacters.
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Run()
	case "linux":
		return exec.Command("xdg-open", path).Run()
	case "android":
		return exec.Command("termux-open", "--view", "--content-type", "image/png", path).Run()
	default:
		return fmt.Errorf("image preview is not supported on %s; use -save or -out", runtime.GOOS)
	}
}
