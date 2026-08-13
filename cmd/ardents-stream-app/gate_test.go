package main

import "testing"

func TestGatedWriterStopsAtExactPrecommittedOffset(t *testing.T) {
	var stored, observed uint32
	write := func(value []byte) (int, error) { stored += uint32(len(value)); return len(value), nil }
	gated := gatedWorkloadWriter(write, 2*16_381, func(offset uint32) error { observed = offset; return nil })
	for range 2 {
		if _, err := gated(make([]byte, 16_381)); err != nil {
			t.Fatal(err)
		}
	}
	if stored != 2*16_381 || observed != stored {
		t.Fatalf("stored %d, gate %d; want exact %d", stored, observed, 2*16_381)
	}
	if _, err := gated([]byte{1}); err != nil {
		t.Fatal(err)
	}
}
