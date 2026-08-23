package nameresolution

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

// BindGatewayState accepts the already-bound Namespace Gateway view and hides
// it together with the Name Authority control port behind one Gateway value.
func BindGatewayState(view *namespace.ResolutionGateway, authority controlAuthority) (gatewayState, error) {
	if view == nil || view.Network() == [32]byte{} {
		return gatewayState{}, errors.New("naming Gateway state is invalid")
	}
	return gatewayState{namespace: view, authority: authority}, nil
}
