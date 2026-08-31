//go:build linux

package installer

import (
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
)

func defaultLocation() location {
	home, err := os.UserHomeDir()
	if err != nil {
		return location{}
	}
	path := filepath.Join(home, ".mozilla", "native-messaging-hosts", browserentry.NativeHostName+".json")
	return location{path: path, register: func(string) error { return nil }, unregister: func(string) error { return nil }}
}
