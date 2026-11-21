// Package generation owns the closed list of generation files, their exclusive
// creation and sealing, and independent retained snapshots of the exact bytes.
// It conveys neither Release authorization nor platform process authority.
package generation

import "errors"

var (
	ErrInput   = errors.New("installation generation: invalid input")
	ErrBinding = errors.New("installation generation: binding mismatch")
)
