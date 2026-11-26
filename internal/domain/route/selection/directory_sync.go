package selection

import (
	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

func syncDirectory(path string) error {
	return durablefile.SyncDirectory(path)
}
