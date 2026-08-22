package nameresolution

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

// BindGatewayState validates and hides the durable Namespace, anonymous-cost,
// and Name Authority infrastructure behind one Gateway-owned state value.
func BindGatewayState(store *namestore.Store, policy namestore.Policy, minimumEpoch uint64,
	epochDigest [32]byte, admission *namespace.Admission, authority controlAuthority,
) (gatewayState, error) {
	if store == nil || admission == nil || epochDigest == [32]byte{} ||
		!validMaterializationPolicy(policy, policy.Network) {
		return gatewayState{}, errors.New("naming Gateway state is invalid")
	}
	return gatewayState{network: policy.Network, recordStore: store,
		policy: cloneMaterializationPolicy(policy), minimum: minimumEpoch,
		epochDigest: epochDigest, admission: admission, authority: authority}, nil
}
