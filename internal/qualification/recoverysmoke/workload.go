package recoverysmoke

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
)

func refreshWorkload(generationRoot string) error {
	for _, name := range []string{"client-seed.hex", "publisher-seed.hex"} {
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(generationRoot, name), []byte(hex.EncodeToString(seed)), 0o600); err != nil {
			return err
		}
	}
	return nil
}
