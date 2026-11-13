package join

import "path/filepath"

type attemptNumber uint32

func (n attemptNumber) directory(root string) string {
	if n != 0 {
		return ""
	}
	return filepath.Join(root, "join-startup")
}
func (n attemptNumber) journal(root string) string {
	if n != 0 {
		return ""
	}
	return root + ".join-first-start-journal"
}
