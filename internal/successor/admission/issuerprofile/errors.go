package issuerprofile

import "errors"

// ErrInvalid means the public profile, binding or cohort inventory is invalid.
var ErrInvalid = errors.New("invalid issuer profile")
