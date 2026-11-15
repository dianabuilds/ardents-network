package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestIntroductionACKIndependentFirstStartBytes(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	reply, err := EncodeIntroductionACK(request, 0)
	want := append(bytes.Clone(request[:]), byte(0))
	if err != nil || !bytes.Equal(reply[:], want) {
		t.Fatal("first-start ACK differs from independent oracle", err)
	}
	for _, index := range []int{0, 32, 159} {
		bad := request
		bad[index] = 'X'
		if _, err := EncodeIntroductionACK(bad, 0); err != ErrInput {
			t.Fatal("noncanonical request accepted", index, err)
		}
	}
	if _, err := EncodeIntroductionACK([160]byte{}, 0); err != ErrInput {
		t.Fatal("zero invocation accepted", err)
	}
	if _, err := (*IntroductionExchange)(nil).Wait(); err != ErrInput {
		t.Fatal("missing original exchange accepted", err)
	}
	if _, err := new(IntroductionExchange).Wait(); err != ErrInput {
		t.Fatal("empty original exchange accepted", err)
	}
	if err := (*IntroductionExchange)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIntroductionACKRetainedChoiceBound(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	reply, err := EncodeIntroductionACK(request, 1)
	if err != nil || !bytes.Equal(reply[:], append(bytes.Clone(request[:]), byte(1))) {
		t.Fatal("retained ACK differs", err)
	}
	for _, choice := range []uint8{2, 255} {
		if _, err := EncodeIntroductionACK(request, choice); err == nil {
			t.Fatal("unknown history choice accepted", choice)
		}
	}
}
