package completion

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestInstallationCompletionFramePreservesClosedASCIIGrammar(t *testing.T) {
	frame := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	if !canonicalFrame(frame) {
		t.Fatal("canonical local frame refused")
	}
	for _, value := range []byte{'A', 'g', 0, '\n', ':', 0xff} {
		changed := frame
		changed[79] = value
		if canonicalFrame(changed) {
			t.Fatal("noncanonical frame accepted", value)
		}
	}
	copy(frame[:32], []byte(strings.Repeat("0", 32)))
	if canonicalFrame(frame) {
		t.Fatal("zero InvocationID accepted")
	}
}

func TestInstallationCompletionEncodePreservesExactClosedBytes(t *testing.T) {
	invocation := [16]byte{0: 1, 15: 255}
	generation, binding := strings.Repeat("a", 64), strings.Repeat("b", 64)
	actual, err := Encode(invocation, generation, binding)
	wanted := []byte("010000000000000000000000000000ff" + generation + binding)
	if err != nil || !bytes.Equal(actual[:], wanted) {
		t.Fatal("completion encoding differs from independent bytes", err)
	}
	actual[0] = 'x'
	fresh, err := Encode(invocation, generation, binding)
	if err != nil || !bytes.Equal(fresh[:], wanted) {
		t.Fatal("returned frame mutation changed later encoding", err)
	}
	for _, invalid := range []string{"", strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("a", 63) + "\n", strings.Repeat("a", 63) + string([]byte{0xff})} {
		for _, fields := range [][2]string{{invalid, binding}, {generation, invalid}} {
			refused, err := Encode(invocation, fields[0], fields[1])
			if !errors.Is(err, ErrInput) || refused != [160]byte{} {
				t.Fatal("invalid digest exposed a completion frame", err)
			}
		}
	}
	refused, err := Encode([16]byte{}, generation, binding)
	if !errors.Is(err, ErrInput) || refused != [160]byte{} {
		t.Fatal("zero invocation exposed a completion frame", err)
	}
}
