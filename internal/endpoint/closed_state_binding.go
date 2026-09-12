package endpoint

import "github.com/dianabuilds/ardents-network/internal/network/state"

// The participant composition supplies the actual opened State owner. Neither
// a local Application nor a permission response can supply this projection.
type closedEndpointState interface {
	CurrentClosedProfile() (state.ClosedProfileView, error)
}
