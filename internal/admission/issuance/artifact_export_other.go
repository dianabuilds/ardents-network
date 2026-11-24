//go:build !linux && !darwin

package issuance

import "context"

// CheckExportDestination refuses state-owned and unavailable destinations before an operation.
func CheckExportDestination(context.Context, string) error { return ErrUnsupported }

// Export publishes the original output bytes after rechecking destination custody.
func Export(context.Context, string, []byte) error { return ErrUnsupported }
