package ardp

import (
	"testing"
	"time"
)

func TestHelloPreservesDistinctAuthorityBindings(t *testing.T) {
	raw := make([]byte, 209)
	for i := 0; i < 5; i++ {
		raw[i*32] = byte(i + 1)
	}
	raw[167] = 1
	raw[168] = 7
	raw[169] = 6
	raw[208] = 1
	got, err := DecodeHello(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkID != ([32]byte{1}) || got.StateGeneration != ([32]byte{2}) ||
		got.StateDigest != ([32]byte{3}) || got.ProfileDigest != ([32]byte{4}) ||
		got.RecipientNodeID != ([32]byte{5}) || !got.Deadline.Equal(time.Unix(1, 0)) {
		t.Fatal("distinct authority bindings swapped", got)
	}
}
