package wire

import (
	"crypto/tls"
	"errors"
	"sync"
)

// TLSBinding seals the original completed TLS object and fresh Attachment
// exporter. It grants no local permission or accepting Publication authority.
type TLSBinding struct {
	mu            sync.RWMutex
	connection    *tls.Conn
	context       [32]byte
	key, exporter [32]byte
}

func BindTLS(connection *tls.Conn, context [32]byte) (*TLSBinding, error) {
	key, exporter, err := AttachmentSecrets(connection, context)
	if err != nil {
		return nil, err
	}
	return &TLSBinding{connection: connection, context: context, key: key, exporter: exporter}, nil
}

// Secrets returns only Connection continuity material, never an Instance key.
func (binding *TLSBinding) Secrets() (key, exporter [32]byte) {
	if binding != nil {
		binding.mu.RLock()
		defer binding.mu.RUnlock()
		return binding.key, binding.exporter
	}
	return key, exporter
}

// Erase retires volatile exporter material after its actual TLS users join.
// Receipts retained by another caller then fail closed rather than resurrect
// their original TLS authority. No filesystem or fresh secret is involved.
func (binding *TLSBinding) Erase() {
	if binding == nil {
		return
	}
	binding.mu.Lock()
	defer binding.mu.Unlock()
	clear(binding.key[:])
	clear(binding.exporter[:])
	binding.connection = nil
}

// InitialReceipt is a one-use verified request tied to the exact TLS object.
// The Instance signing owner independently matches its expected challenge
// fields and the original TLS binding before consuming this receipt.
type InitialReceipt struct {
	mu             sync.Mutex
	binding        *TLSBinding
	challenge      Challenge
	peerNonce      [32]byte
	used           bool
	responseIssued bool
	self           *InitialReceipt
}

// VerifyInitialRequest checks the coalesced request before Instance key use.
// The Publisher learns the fresh challenge nonce from this request; all other
// expected fields come from its original independently admitted logical tuple.
func VerifyInitialRequest(binding *TLSBinding, expected Challenge, challenge Challenge, continuity Continuity) (*InitialReceipt, error) {
	if binding == nil || expected.Nonce != [32]byte{} {
		return nil, errors.New("initial TLS request provenance absent")
	}
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	if binding.connection == nil || binding.key == [32]byte{} || binding.exporter == [32]byte{} {
		return nil, errors.New("initial TLS binding retired")
	}
	expected.Nonce = challenge.Nonce
	if challenge != expected || challenge.Nonce == [32]byte{} || challenge.Nonce == continuity.Nonce {
		return nil, errors.New("initial Instance challenge differs")
	}
	if _, err := ChallengeDigest(challenge); err != nil {
		return nil, err
	}
	if err := VerifyContinuity(binding.key, continuity, RoleInitiator, 1, expected.Context, binding.exporter); err != nil {
		return nil, err
	}
	if continuity.SendBase != 0 || continuity.SendEnd != 0 || continuity.ReceiveNext != 0 {
		return nil, errors.New("initial Continuity offsets differ")
	}
	receipt := &InitialReceipt{binding: binding, challenge: challenge, peerNonce: continuity.Nonce}
	receipt.self = receipt
	return receipt, nil
}

// Consume denies copied/foreign TLS identity and repeated purpose signing.
// Its expected nonce is absent because the checked request owns that nonce.
func (receipt *InitialReceipt) Consume(binding *TLSBinding, expected Challenge) (Challenge, error) {
	if receipt == nil || binding == nil || expected.Nonce != [32]byte{} {
		return Challenge{}, errors.New("initial request receipt absent")
	}
	receipt.mu.Lock()
	defer receipt.mu.Unlock()
	binding.mu.RLock()
	defer binding.mu.RUnlock()
	expected.Nonce = receipt.challenge.Nonce
	if binding.connection == nil || receipt.self != receipt || receipt.used || receipt.binding != binding || receipt.challenge != expected {
		return Challenge{}, errors.New("initial request receipt foreign or consumed")
	}
	receipt.used = true
	return receipt.challenge, nil
}
