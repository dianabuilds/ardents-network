package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestIntroductionACKIndependentFirstStartBytes(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	reply, err := EncodeIntroductionACK(request)
	want := append(bytes.Clone(request[:]), byte(0))
	if err != nil || !bytes.Equal(reply[:], want) {
		t.Fatal("first-start ACK differs from independent oracle", err)
	}
	for _, index := range []int{0, 32, 159} {
		bad := request
		bad[index] = 'X'
		if _, err := EncodeIntroductionACK(bad); err != ErrInput {
			t.Fatal("noncanonical request accepted", index, err)
		}
	}
	if _, err := EncodeIntroductionACK([160]byte{}); err != ErrInput {
		t.Fatal("zero invocation accepted", err)
	}
	if err := (*IntroductionExchange)(nil).Wait(); err != ErrInput {
		t.Fatal("missing original exchange accepted", err)
	}
	if err := new(IntroductionExchange).Wait(); err != ErrInput {
		t.Fatal("empty original exchange accepted", err)
	}
	if err := (*IntroductionExchange)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}
