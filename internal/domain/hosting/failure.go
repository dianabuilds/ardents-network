package hosting

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting/counters"
)

// Stable failure categories never include selected paths or observed metadata.
var (
	ErrUnsupportedPlatform = counters.ErrUnsupportedPlatform
	ErrInvalid             = errors.New("invalid-input")
	ErrUnavailable         = counters.ErrUnavailable
	ErrCapacity            = errors.New("budget-exhausted")
	ErrUncertain           = errors.New("storage-uncertain")
	ErrReservationInUse    = errors.New("reservation-in-use")
)

// Supported reports the platform prerequisite without reading inputs or storage.
func Supported() bool { return counters.Supported() }
