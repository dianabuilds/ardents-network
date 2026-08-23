package namespace

import "errors"

// ResolutionGateway is the Namespace-owned server view for one private
// resolution Gateway. It keeps the durable state, epoch trust root, and
// one-use admission gate together so the transport owner cannot recombine
// their fields or bypass a verification step.
type ResolutionGateway struct {
	store        *Store
	policy       MaterializationPolicy
	minimumEpoch uint64
	epochDigest  [32]byte
	admission    *Admission
}

// OpenResolutionGateway binds one current Namespace Store to its trusted
// epoch and Gateway-local admission gate. Store policy is copied from the
// already-open Store: a caller cannot pair a Store with another trust root.
func OpenResolutionGateway(store *Store, minimumEpoch uint64, epochDigest [32]byte,
	admission *Admission,
) (*ResolutionGateway, error) {
	if store == nil || store.root == nil || admission == nil || minimumEpoch == 0 || epochDigest == [32]byte{} ||
		admission.network != store.policy.Network || admission.epoch != minimumEpoch {
		return nil, errors.New("naming resolution Gateway is invalid")
	}
	policy, err := validMaterializationPolicy(store.policy)
	if err != nil {
		return nil, errors.New("naming resolution Gateway policy is invalid")
	}
	return &ResolutionGateway{store: store, policy: policy, minimumEpoch: minimumEpoch,
		epochDigest: epochDigest, admission: admission}, nil
}

// Network returns the fixed Network identity trusted by this Gateway view.
func (gateway *ResolutionGateway) Network() [32]byte {
	if gateway == nil {
		return [32]byte{}
	}
	return gateway.policy.Network
}

// AcceptsGateway reports whether this view's admission gate belongs to the
// configured Gateway Node. It prevents a role from consuming another Node's
// boot-scoped proof state.
func (gateway *ResolutionGateway) AcceptsGateway(node [32]byte) bool {
	return gateway != nil && gateway.admission != nil && node != [32]byte{} && gateway.admission.node == node
}

// AdmitResolution consumes one proof only when it is bound to this Gateway,
// the exact private-resolution operation, and the accepted surface.
func (gateway *ResolutionGateway) AdmitResolution(now int64, node, operation [32]byte, proof Proof) bool {
	if gateway == nil || gateway.admission == nil || node == [32]byte{} || operation == [32]byte{} ||
		proof.Challenge.Node != node || proof.Challenge.Surface != "resolution" ||
		proof.Challenge.OperationDigest != operation {
		return false
	}
	accepted, _ := gateway.admission.Verify(now, proof)
	return accepted
}

// LookupBinding returns a compact proof and its already-authenticated
// destination binding. An unavailable, stale, or tampered Store has no
// binding result and never exposes its lifecycle Record to Resolution.
func (gateway *ResolutionGateway) LookupBinding(name string, at int64) ([]byte, Binding, bool) {
	if gateway == nil || gateway.store == nil {
		return nil, Binding{}, false
	}
	proof, err := gateway.store.Lookup(name, gateway.minimumEpoch)
	if err != nil {
		return nil, Binding{}, false
	}
	binding, _, _, err := VerifyBinding(gateway.policy, proof, gateway.minimumEpoch, gateway.epochDigest, at)
	if err != nil {
		return nil, Binding{}, false
	}
	return proof, binding, true
}

// ResolutionVerifier is the Namespace-owned client view for one authenticated
// Network Epoch. It verifies only immutable destination bindings.
type ResolutionVerifier struct {
	policy       MaterializationPolicy
	minimumEpoch uint64
	epochDigest  [32]byte
}

// OpenResolutionVerifier validates and copies one selected Namespace trust
// root for a private-resolution client.
func OpenResolutionVerifier(input MaterializationPolicy, minimumEpoch uint64,
	epochDigest [32]byte,
) (*ResolutionVerifier, error) {
	policy, err := validMaterializationPolicy(input)
	if err != nil || minimumEpoch == 0 || epochDigest == [32]byte{} {
		return nil, errors.New("naming resolution verifier is invalid")
	}
	return &ResolutionVerifier{policy: policy, minimumEpoch: minimumEpoch, epochDigest: epochDigest}, nil
}

// Verify authenticates one received proof and returns only its immutable
// destination binding; the signed lifecycle Record remains internal.
func (verifier *ResolutionVerifier) Verify(proof []byte, at int64) (Binding, string, error) {
	if verifier == nil {
		return Binding{}, "", errors.New("naming resolution verifier is unavailable")
	}
	binding, warning, _, err := VerifyBinding(verifier.policy, proof, verifier.minimumEpoch,
		verifier.epochDigest, at)
	return binding, warning, err
}
