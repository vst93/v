//go:build linux

package plugin_save

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func defaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	if dir := xdgDownloadDir(home); dir != "" {
		return dir
	}
	return filepath.Join(home, "Downloads")
}

func xdgDownloadDir(home string) string {
	data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR=") {
			continue
		}
		value := strings.Trim(line[len("XDG_DOWNLOAD_DIR="):], "\"")
		value = strings.ReplaceAll(value, "$HOME", home)
		if value != "" {
			return expandHome(value)
		}
	}
	return ""
}

func revealFile(path string) error {
	// Ask the desktop's FileManager1 (GNOME/KDE) to reveal the file first.
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	dbus := exec.Command("dbus-send", "--session", "--print-reply",
		"--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems", "array:string:"+uri, "string:")
	if err := dbus.Run(); err == nil {
		return nil
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Run()
}
