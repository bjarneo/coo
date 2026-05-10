// Package appdir resolves the coo user configuration directory.
package appdir

import (
	"os"
	"path/filepath"
)

// Dir returns ~/.config/coo.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "coo"), nil
}
