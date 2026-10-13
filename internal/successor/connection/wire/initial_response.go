package wire

import (
	"bytes"
	"errors"
	"sync"
)

// InitialResponse is a sealed one-use canonical coalesced response. Its exact
// TLS binding, verified request and actual Instance proof cannot be substituted
// by an established flag or an arbitrary successful signer callback.
type InitialResponse struct {
	mu      sync.Mutex
	self    *InitialResponse
	binding *TLSBinding
	proof   Proof
	body    []byte
	used    bool
}

func NewInitialResponse(receipt *InitialReceipt, instance [32]byte, proof Proof, continuity Continuity) (*InitialResponse, error) {
	if receipt == nil || instance == [32]byte{} {
		return nil, errors.New("initial response request absent")
	}
	receipt.mu.Lock()
	defer receipt.mu.Unlock()
	if receipt.self != receipt || !receipt.used || receipt.responseIssued || receipt.binding == nil {
		return nil, errors.New("initial request not consumed by Instance or response already issued")
	}
	receipt.binding.mu.RLock()
	defer receipt.binding.mu.RUnlock()
	if receipt.binding.connection == nil || receipt.binding.key == [32]byte{} {
		return nil, errors.New("initial response TLS retired")
	}
	if err := verifyInstanceProof(instance, receipt.challenge, proof); err != nil {
		return nil, errors.Join(errors.New("initial Instance response proof differs"), err)
	}
	if err := VerifyContinuity(receipt.binding.key, continuity, RolePublisher, 1, receipt.challenge.Context, receipt.binding.exporter); err != nil {
		return nil, err
	}
	if continuity.SendBase != 0 || continuity.SendEnd != 0 || continuity.ReceiveNext != 0 || continuity.Nonce == receipt.challenge.Nonce || continuity.Nonce == receipt.peerNonce {
		return nil, errors.New("initial Publisher offsets or nonce differ")
	}
	var body bytes.Buffer
	if err := Write(&body, Record{Proof: &proof}); err != nil {
		return nil, err
	}
	if err := Write(&body, Record{Continuity: &continuity}); err != nil {
		return nil, err
	}
	response := &InitialResponse{binding: receipt.binding, proof: proof, body: body.Bytes()}
	response.self = response
	receipt.responseIssued = true
	return response, nil
}

// Consume returns a copied finite flight only to its exact TLS signing owner.
func (response *InitialResponse) Consume(binding *TLSBinding, proof Proof) ([]byte, error) {
	if response == nil || binding == nil {
		return nil, errors.New("initial response absent")
	}
	response.mu.Lock()
	defer response.mu.Unlock()
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if binding.connection == nil || response.self != response || response.used || response.binding != binding || response.proof != proof {
		return nil, errors.New("initial response foreign or consumed")
	}
	response.used = true
	return append([]byte(nil), response.body...), nil
}
