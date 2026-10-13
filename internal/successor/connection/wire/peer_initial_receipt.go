package wire

import (
	"errors"
	"sync"
)

// PeerInitialReceipt seals the independently verified Publisher response to
// the Reader's exact TLS binding and original coalesced request.
type PeerInitialReceipt struct {
	mu        sync.Mutex
	self      *PeerInitialReceipt
	binding   *TLSBinding
	challenge Challenge
	used      bool
}

func VerifyPeerInitialResponse(binding *TLSBinding, instance [32]byte, challenge Challenge, request Continuity, proof Proof, response Continuity) (*PeerInitialReceipt, error) {
	if binding == nil || instance == [32]byte{} {
		return nil, errors.New("original Reader TLS response is absent")
	}
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if binding.connection == nil || binding.key == [32]byte{} || binding.exporter == [32]byte{} {
		return nil, errors.New("reader TLS binding is retired")
	}
	if err := verifyInstanceProof(instance, challenge, proof); err != nil {
		return nil, errors.Join(errors.New("publisher Instance proof differs"), err)
	}
	if err := VerifyContinuity(binding.key, request, RoleInitiator, 1, challenge.Context, binding.exporter); err != nil {
		return nil, err
	}
	if err := VerifyContinuity(binding.key, response, RolePublisher, 1, challenge.Context, binding.exporter); err != nil {
		return nil, err
	}
	if request.SendBase != 0 || request.SendEnd != 0 || request.ReceiveNext != 0 || response.SendBase != 0 || response.SendEnd != 0 || response.ReceiveNext != 0 ||
		challenge.Nonce == request.Nonce || challenge.Nonce == response.Nonce || request.Nonce == response.Nonce {
		return nil, errors.New("initial response offsets or nonce differ")
	}
	receipt := &PeerInitialReceipt{binding: binding, challenge: challenge}
	receipt.self = receipt
	return receipt, nil
}

func (receipt *PeerInitialReceipt) Consume(binding *TLSBinding, challenge Challenge) error {
	if receipt == nil || binding == nil {
		return errors.New("verified Reader initial response is absent")
	}
	receipt.mu.Lock()
	defer receipt.mu.Unlock()
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if receipt.self != receipt || receipt.used || receipt.binding != binding || receipt.challenge != challenge || binding.connection == nil {
		return errors.New("reader initial response is foreign or consumed")
	}
	receipt.used = true
	return nil
}
