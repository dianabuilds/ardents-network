package enrollment

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckProtectedGenerationFiles checks the static producer inputs using the
// signed inventory names. The two executable files can come from separate
// build locations. This check supplies no initial pin or Release authority.
func CheckProtectedGenerationFiles(root, endpoint, documentWorker, releaseIdentity string) error {
	paths := map[string]string{
		"ardents-linux-amd64":      endpoint,
		"ardents-text-linux-amd64": documentWorker,
	}
	names := protectedNames()
	files := make(map[string][]byte, len(names))
	for _, name := range names {
		path, supplied := paths[name]
		if !supplied {
			path = filepath.Join(root, name)
		}
		raw, err := readProtectedGenerationFile(path, 64<<20)
		if err != nil {
			return err
		}
		files[name] = raw
	}
	descriptor, err := readProtectedGenerationFile(filepath.Join(root, "protected-endpoint.json"), 16<<10)
	if err != nil {
		return err
	}
	return ValidateProtectedGeneration(descriptor, files, releaseIdentity)
}

func readProtectedGenerationFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, fmt.Errorf("protected producer input is not a bounded direct regular file: %s", path)
	}
	return os.ReadFile(path)
}
