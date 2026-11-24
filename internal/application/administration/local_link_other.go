//go:build !linux

package administration

import (
	"context"
	"errors"
)

// PresentLocalLink reports that local Link presentation is unavailable.
func PresentLocalLink(context.Context, string) error {
	return errors.New("document Link presentation requires the selected Linux host")
}
