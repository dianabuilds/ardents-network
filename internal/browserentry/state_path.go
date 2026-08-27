package browserentry

import (
	"os"
	"path/filepath"
)

// DefaultStatePath is the fixed per-user state location expected by the
// installed Firefox native-host manifest. A qualification invocation may supply
// a different absolute path, but a released native manifest needs no arguments.
func DefaultStatePath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Ardents", "browser-entry", "alpha-proxy.json"), nil
}
