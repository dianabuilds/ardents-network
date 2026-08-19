package blockedentry

import "testing"

func TestFinalG4ReceiptDistinguishesAtomicAndTerminalPhases(t *testing.T) {
	atomic := []byte(`{"schema":"ardents-h3-g4-receipt-v1","phase":"after-regime-publication","checkpoint":{"regime":true,"attempt":true,"contacts":1},"reopened":{"kind":"g4-reopen","phase":"after-regime-publication","terminal":"bridge-interrupted","attempt":true,"contacts":1},"atomic_with":"after-exposure-0","observation":"durable-generation"}`)
	if !validFinalG4Receipt(atomic, "after-regime-publication") {
		t.Fatal("atomic regime/exposure receipt was rejected")
	}
	terminal := []byte(`{"schema":"ardents-h3-g4-receipt-v1","phase":"after-terminal-record","checkpoint":{"regime":true,"attempt":true,"contacts":1,"terminal":"opened"},"reopened":{"kind":"g4-reopen","phase":"after-terminal-record","terminal":"opened","attempt":true,"contacts":1},"observation":"terminal-generation"}`)
	if !validFinalG4Receipt(terminal, "after-terminal-record") {
		t.Fatal("preserved terminal receipt was rejected")
	}
	terminal = []byte(`{"schema":"ardents-h3-g4-receipt-v1","phase":"after-terminal-record","checkpoint":{"regime":true,"attempt":true,"contacts":1,"terminal":"opened"},"reopened":{"kind":"g4-reopen","phase":"after-terminal-record","terminal":"bridge-interrupted","attempt":true,"contacts":1},"observation":"terminal-generation"}`)
	if validFinalG4Receipt(terminal, "after-terminal-record") {
		t.Fatal("rewritten terminal receipt was accepted")
	}
}
