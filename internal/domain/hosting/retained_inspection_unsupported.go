//go:build !linux

package hosting

import "context"

// Inspection is unavailable off Linux. The retained-history inspection requires
// the original root-only lease and the Linux ext4 storage floor (PO decision
// P1.1, #865); macOS APFS would need a separately researched floor, so every
// other platform receives an explicit typed refusal rather than a weaker path.
type Inspection struct{}

func OpenInspection(context.Context, string, string, uint32, uint32) (Inspection, error) {
	return Inspection{}, ErrUnsupportedPlatform
}

func (Inspection) Check() error { return ErrUnsupportedPlatform }

func (Inspection) Close() error { return ErrUnsupportedPlatform }
