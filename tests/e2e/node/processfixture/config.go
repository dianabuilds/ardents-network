package processfixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// WriteConfig writes one scenario’s private JSON command input.
func WriteConfig(t testing.TB, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
