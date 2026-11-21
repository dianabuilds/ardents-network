package wire

import (
	"errors"
	"sync"
)

// ReplacementReceipt proves only the exact fresh TLS/Continuity transcript.
// Original Connection authority, byte ranges, proposal bounds and joined
// retirement remain with Connection; copied counters confer none of them.
type ReplacementReceipt struct {
	mu             sync.Mutex
	self           *ReplacementReceipt
	binding        *TLSBinding
	peer           Continuity
	keyCommitment  [32]byte
	used           bool
	responseIssued bool
}

// VerifyReplacement uses the ORIGINAL logical key and this new Attachment's
// exporter. The fresh TLS-derived key never becomes a replacement secret.
func VerifyReplacement(binding *TLSBinding, originalKey [32]byte, peer Continuity, role Role, generation uint64, logical [32]byte) (*ReplacementReceipt, error) {
	if binding == nil || generation < 2 {
		return nil, errors.New("replacement TLS provenance absent")
	}
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if binding.connection == nil || binding.context == [32]byte{} || binding.key == [32]byte{} || binding.exporter == [32]byte{} {
		return nil, errors.New("replacement TLS binding retired")
	}
	if err := VerifyContinuity(originalKey, peer, role, generation, logical, binding.exporter); err != nil {
		return nil, err
	}
	receipt := &ReplacementReceipt{binding: binding, peer: peer, keyCommitment: ContinuityCommitment(originalKey)}
	receipt.self = receipt
	return receipt, nil
}

// Consume returns authenticated peer facts once to the exact original-key,
// logical/generation and TLS consumer. It performs no byte-state mutation.
func (receipt *ReplacementReceipt) Consume(binding *TLSBinding, role Role, generation uint64, logical, keyCommitment [32]byte) (Continuity, error) {
	if receipt == nil || binding == nil || generation < 2 || !validRole(role) || logical == [32]byte{} || keyCommitment == [32]byte{} {
		return Continuity{}, errors.New("replacement receipt absent")
	}
	receipt.mu.Lock()
	defer receipt.mu.Unlock()
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if receipt.self != receipt || receipt.used || receipt.binding != binding || binding.connection == nil || binding.key == [32]byte{} ||
		binding.exporter != receipt.peer.ExporterCommitment || receipt.peer.Role != role || receipt.peer.AttachmentGeneration != generation ||
		receipt.peer.Context != logical || receipt.keyCommitment != keyCommitment {
		return Continuity{}, errors.New("replacement receipt foreign or consumed")
	}
	receipt.used = true
	return receipt.peer, nil
}
