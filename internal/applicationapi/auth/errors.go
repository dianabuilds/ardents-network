// Package auth defines Application Interface authentication outcomes.
package auth

import "errors"

var (
	ErrUnauthenticated = errors.New("application authentication failed")
	ErrForbidden       = errors.New("application action is forbidden")
)
