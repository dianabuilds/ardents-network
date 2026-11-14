package descriptor

import "path/filepath"

type attemptNumber uint32

func (n attemptNumber) directory(root string) string {
	if n != 0 {
		return ""
	}
	return filepath.Join(root, "descriptor-startup")
}
func (n attemptNumber) journal(root string) string {
	if n != 0 {
		return ""
	}
	return root + ".descriptor-first-start-journal"
}
