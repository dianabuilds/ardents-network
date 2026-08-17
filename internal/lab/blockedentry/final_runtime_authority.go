package blockedentry

import (
	"os"
	"path/filepath"
)

const finalDockerConfigPath = "runtime/docker-config/config.json"

func freezeFinalRuntimeAuthority(secretRoot string) error {
	path := filepath.Join(secretRoot, filepath.FromSlash(finalDockerConfigPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("{}\n"), 0o400)
}
