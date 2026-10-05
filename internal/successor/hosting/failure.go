package hosting

import "errors"

// Stable failure categories never include selected paths or observed metadata.
var (
	ErrUnsupportedPlatform = errors.New("unsupported-platform")
	ErrInvalid             = errors.New("invalid-input")
	ErrUnavailable         = errors.New("budget-unavailable")
	ErrCapacity            = errors.New("budget-exhausted")
	ErrUncertain           = errors.New("storage-uncertain")
	ErrReservationInUse    = errors.New("reservation-in-use")
)

// Supported reports the platform prerequisite without reading inputs or storage.
func Supported() bool { return hostingPlatform() == nil }
