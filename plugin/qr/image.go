package plugin_qr

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func saveQRImage(data []byte, opts qrOptions) (string, error) {
	if opts.out != "" {
		path, err := filepath.Abs(expandHome(opts.out))
		if err != nil {
			return "", fmt.Errorf("resolve output path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", fmt.Errorf("write PNG: %w", err)
		}
		return path, nil
	}
	dir := os.TempDir()
	if opts.save {
		var err error
		dir, err = defaultDownloadsDir()
		if err != nil {
			return "", err
		}
	}
	return writeUniquePNG(dir, data)
}

func writeUniquePNG(dir string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create image directory: %w", err)
	}
	// Unique names allow repeated saves without replacing an earlier QR code.
	file, err := os.CreateTemp(dir, "qr_"+time.Now().Format("20060102_150405")+"_*.png")
	if err != nil {
		return "", fmt.Errorf("create PNG: %w", err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return "", fmt.Errorf("write PNG %s: %w", file.Name(), writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close PNG %s: %w", file.Name(), closeErr)
	}
	// Keep preview files: desktop openers may return before the viewer reads them.
	return file.Name(), nil
}
