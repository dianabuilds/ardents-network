//go:build !linux && !darwin

package introduction

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/spending"
)

// errHistoryPlatformUnavailable is the explicit typed unavailable outcome for
// the introduction slot-history root on platforms without the selected
// Unix-family durable mechanism (ADR-0129). A missing required protection
// yields the same explicit refusal on every platform; it never supplies a
// weaker accepting path.
var errHistoryPlatformUnavailable = errors.New("introduction history platform unavailable")

// OpenHistory refuses on platforms without the selected Unix-family durable
// root mechanism; it never creates, resets or weakly accepts a retained root.
func OpenHistory(root string, binding Binding) (*History, error) {
	return nil, errHistoryPlatformUnavailable
}

// InitializeHistory refuses on platforms without the selected Unix-family
// durable root mechanism; it consumes no fresh-root fact and creates nothing.
func InitializeHistory(root string, binding Binding, fact *spending.FreshRoot) (*History, error) {
	return nil, errHistoryPlatformUnavailable
}
