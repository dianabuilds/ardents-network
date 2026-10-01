//go:build !windows

package publication

import (
	"errors"
	"fmt"
	"os"
)

func syncPublicationDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open publication directory barrier: %w", err)
	}
	return errors.Join(directory.Sync(), directory.Close())
}
