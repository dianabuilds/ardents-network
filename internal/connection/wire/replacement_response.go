package wire

import (
	"bytes"
	"errors"
	"sync"
)

// ReplacementResponse seals one canonical Continuity flight under the same
// original key as the verified peer. It is not an Application handoff receipt.
type ReplacementResponse struct {
	mu            sync.Mutex
	self          *ReplacementResponse
	binding       *TLSBinding
	local         Continuity
	keyCommitment [32]byte
	body          []byte
	used          bool
}

func NewReplacementResponse(receipt *ReplacementReceipt, originalKey [32]byte, local Continuity) (*ReplacementResponse, error) {
	if receipt == nil || originalKey == [32]byte{} {
		return nil, errors.New("replacement response request absent")
	}
	receipt.mu.Lock()
	defer receipt.mu.Unlock()
	if receipt.self != receipt || !receipt.used || receipt.responseIssued || receipt.binding == nil || receipt.keyCommitment != ContinuityCommitment(originalKey) {
		return nil, errors.New("replacement request unconsumed or original key differs")
	}
	binding := receipt.binding
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if binding.connection == nil || binding.key == [32]byte{} || binding.exporter != receipt.peer.ExporterCommitment {
		return nil, errors.New("replacement response TLS retired")
	}
	role := RoleInitiator
	if receipt.peer.Role == RoleInitiator {
		role = RolePublisher
	}
	if err := VerifyContinuity(originalKey, local, role, receipt.peer.AttachmentGeneration, receipt.peer.Context, binding.exporter); err != nil {
		return nil, err
	}
	if local.Nonce == receipt.peer.Nonce {
		return nil, errors.New("replacement peer and local nonce coincide")
	}
	var body bytes.Buffer
	if err := Write(&body, Record{Continuity: &local}); err != nil {
		return nil, err
	}
	response := &ReplacementResponse{binding: binding, local: local, keyCommitment: receipt.keyCommitment, body: body.Bytes()}
	response.self = response
	receipt.responseIssued = true
	return response, nil
}

func (response *ReplacementResponse) Consume(binding *TLSBinding, role Role, generation uint64, logical, keyCommitment [32]byte) ([]byte, error) {
	if response == nil || binding == nil || generation < 2 || !validRole(role) || logical == [32]byte{} || keyCommitment == [32]byte{} {
		return nil, errors.New("replacement response absent")
	}
	response.mu.Lock()
	defer response.mu.Unlock()
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if response.self != response || response.used || response.binding != binding || binding.connection == nil || binding.key == [32]byte{} ||
		binding.exporter != response.local.ExporterCommitment || response.local.Role != role || response.local.AttachmentGeneration != generation ||
		response.local.Context != logical || response.keyCommitment != keyCommitment {
		return nil, errors.New("replacement response foreign or consumed")
	}
	response.used = true
	return append([]byte(nil), response.body...), nil
}
