package wire

import (
	"crypto/tls"
	"testing"
)

// This private fixture isolates receipt ownership, not completed TLS authority.
// Production TLSBinding is issued only by BindTLS after actual TLS1.3.
func TestInitialReceiptRejectsCopyAndForeignFields(t *testing.T) {
	t.Parallel()
	binding := &TLSBinding{connection: &tls.Conn{}, key: [32]byte{1}, exporter: [32]byte{2}}
	expected := Challenge{Network: [32]byte{3}, Target: [32]byte{4}, InstanceGeneration: 5, Context: [32]byte{6}}
	challenge := expected
	challenge.Nonce = [32]byte{7}
	continuity, err := NewContinuity(binding.key, RoleInitiator, 1, 0, 0, 0, expected.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := VerifyInitialRequest(binding, expected, challenge, continuity)
	if err != nil {
		t.Fatal(err)
	}
	copied := &InitialReceipt{binding: receipt.binding, challenge: receipt.challenge, self: receipt.self}
	if _, err := copied.Consume(binding, expected); err == nil {
		t.Fatal("copied signing receipt accepted")
	}
	foreign := expected
	foreign.Target[0]++
	if _, err := receipt.Consume(binding, foreign); err == nil {
		t.Fatal("foreign Target consumed signing receipt")
	}
	if _, err := receipt.Consume(binding, expected); err != nil {
		t.Fatal("refusal consumed original receipt", err)
	}
	if _, err := receipt.Consume(binding, expected); err == nil {
		t.Fatal("same signing receipt reused")
	}
	binding.Erase()
	if key, exporter := binding.Secrets(); key != [32]byte{} || exporter != [32]byte{} {
		t.Fatal("retired exporter secrets survived")
	}
	if _, err := VerifyInitialRequest(binding, expected, challenge, continuity); err == nil {
		t.Fatal("erased TLS supplied new signing receipt")
	}
}
