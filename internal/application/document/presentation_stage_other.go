//go:build !linux

package document

import (
	"errors"
	"os"
)

func openPresentationStage() (*os.File, error) {
	return nil, errors.New("private document presentation storage unavailable")
}
