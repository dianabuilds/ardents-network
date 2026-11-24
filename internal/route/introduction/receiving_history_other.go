//go:build !linux && !darwin

package introduction

import (
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
)

// OpenReceivingRegistry refuses before consuming any fresh-root fact on
// platforms without the selected Unix-family durable root mechanism
// (ADR-0129); it creates no History and grants no capacity.
func OpenReceivingRegistry(root string, binding Binding, receivingOwner *receiving.Owner) (*Registry, error) {
	return nil, errHistoryPlatformUnavailable
}

// OpenRetainedReceivingRegistry refuses on platforms without the selected
// Unix-family durable root mechanism; it never recreates erased slot/time
// floors.
func OpenRetainedReceivingRegistry(root string, binding Binding) (*Registry, error) {
	return nil, errHistoryPlatformUnavailable
}
