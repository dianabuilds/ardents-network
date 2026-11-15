package main

import (
	"crypto/sha256"
	"testing"
)

func TestNodeAuthorityPinsRejectsZeroAndDuplicatesAndDetachesKeys(t *testing.T) {
	key := [32]byte{1}
	values := [][32]byte{key}
	pins, ok := nodeAuthorityPins(values)
	id := sha256.Sum256(key[:])
	if !ok || len(pins) != 1 || len(pins[id]) != 32 || pins[id][0] != 1 {
		t.Fatal("original authority not projected")
	}
	values[0][0] = 2
	if pins[id][0] != 1 {
		t.Fatal("authority pin aliases declaration")
	}
	for _, input := range [][][32]byte{{{}}, {key, key}} {
		if _, ok := nodeAuthorityPins(input); ok {
			t.Fatal("invalid authority accepted")
		}
	}
}
