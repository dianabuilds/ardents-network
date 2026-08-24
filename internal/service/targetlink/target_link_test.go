package targetlink

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	link := Link{Network: bytes(1), Target: bytes(33)}
	text, err := Encode(link)
	if err != nil {
		t.Fatal(err)
	}
	if text != "ardents-target:v1:AQECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0-P0A" {
		t.Fatalf("Target Link = %q", text)
	}
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if got != link {
		t.Fatalf("decoded Link = %#v, want %#v", got, link)
	}
}

func TestEncodeRejectsEmptyProtocolValues(t *testing.T) {
	for _, link := range []Link{{}, {Network: bytes(1)}, {Target: bytes(33)}} {
		if text, err := Encode(link); !errors.Is(err, ErrFormat) || text != "" {
			t.Fatalf("Encode(%#v) = (%q, %v)", link, text, err)
		}
	}
}

func TestDecodeRejectsNonCanonicalAndForeignText(t *testing.T) {
	valid, err := Encode(Link{Network: bytes(1), Target: bytes(33)})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(valid, prefix))
	if err != nil {
		t.Fatal(err)
	}
	unknown := append([]byte(nil), payload...)
	unknown[algorithmOffset] = 2
	cases := []struct {
		name string
		text string
		err  error
	}{
		{"foreign-prefix", strings.Replace(valid, prefix, "ardents://", 1), ErrFormat},
		{"padding", valid + "=", ErrFormat},
		{"wrong-size", prefix + base64.RawURLEncoding.EncodeToString(payload[:64]), ErrFormat},
		{"unknown-algorithm", prefix + base64.RawURLEncoding.EncodeToString(unknown), ErrAlgorithm},
		{"whitespace", valid + "\n", ErrFormat},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got, err := Decode(test.text); !errors.Is(err, test.err) || got != (Link{}) {
				t.Fatalf("Decode(%q) = (%#v, %v)", test.text, got, err)
			}
		})
	}
}

func TestDecodeRejectsZeroProtocolValues(t *testing.T) {
	for _, link := range []Link{{Network: bytes(1)}, {Target: bytes(33)}} {
		payload := make([]byte, payloadSize)
		payload[algorithmOffset] = targetAlgorithm
		copy(payload[networkOffset:targetOffset], link.Network[:])
		copy(payload[targetOffset:], link.Target[:])
		if got, err := Decode(prefix + base64.RawURLEncoding.EncodeToString(payload)); !errors.Is(err, ErrFormat) || got != (Link{}) {
			t.Fatalf("Decode zero value = (%#v, %v)", got, err)
		}
	}
}

func bytes(start byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = start + byte(index)
	}
	return result
}
