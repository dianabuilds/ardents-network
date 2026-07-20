package recorder

import "path/filepath"

func LedgerPath(dir string) string {
	return filepath.Join(dir, "operations.json")
}
