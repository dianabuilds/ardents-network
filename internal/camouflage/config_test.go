package camouflage_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

func TestValidateAcceptsOnlyCanonicalWebTunnelEnvelope(t *testing.T) {
	t.Parallel()
	identity := [32]byte{1, 2, 3}
	valid := candidateEnvelope()
	config, err := camouflage.Validate(valid, identity)
	if err != nil {
		t.Fatalf("validate canonical envelope: %v", err)
	}
	if config.Commitment() == ([32]byte{}) {
		t.Fatal("canonical envelope has zero commitment")
	}
	for _, raw := range [][]byte{nil, make([]byte, 1025)} {
		if _, err := camouflage.Validate(raw, identity); err == nil || err.Error() != "adapter-config-invalid" {
			t.Fatalf("out-of-bound envelope error = %v", err)
		}
	}

	tests := map[string]func([]byte) []byte{
		"trailing":      func(raw []byte) []byte { return append(raw, 0) },
		"wrong magic":   func(raw []byte) []byte { raw[0] ^= 1; return raw },
		"wrong version": func(raw []byte) []byte { raw[len("ardents-h3-wt1")] = 2; return raw },
		"wrong profile": func(raw []byte) []byte { raw[len("ardents-h3-wt1")+2] ^= 1; return raw },
		"private address": func(raw []byte) []byte {
			offset := len("ardents-h3-wt1") + 1 + 1 + len("webtunnel-v0.0.6")
			copy(raw[offset:offset+4], []byte{10, 0, 0, 1})
			return raw
		},
		"zero port": func(raw []byte) []byte {
			offset := len("ardents-h3-wt1") + 1 + 1 + len("webtunnel-v0.0.6") + 4
			clear(raw[offset : offset+2])
			return raw
		},
		"root path":      func(raw []byte) []byte { return replaceCandidatePath(raw, []byte("/")) },
		"query path":     func(raw []byte) []byte { return replaceCandidatePath(raw, []byte("/entry?q=1")) },
		"fragment path":  func(raw []byte) []byte { return replaceCandidatePath(raw, []byte("/entry#x")) },
		"backslash path": func(raw []byte) []byte { return replaceCandidatePath(raw, []byte("/entry\\x")) },
		"uppercase name": func(raw []byte) []byte { return replaceCandidateName(raw, []byte("Front.Example")) },
		"trailing dot":   func(raw []byte) []byte { return replaceCandidateName(raw, []byte("front.example.")) },
		"empty label":    func(raw []byte) []byte { return replaceCandidateName(raw, []byte("front..example")) },
		"IP name":        func(raw []byte) []byte { return replaceCandidateName(raw, []byte("192.0.2.1")) },
		"zero pin":       func(raw []byte) []byte { clear(raw[len(raw)-32:]); return raw },
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := mutate(bytes.Clone(valid))
			if _, err := camouflage.Validate(raw, identity); err == nil || err.Error() != "adapter-config-invalid" {
				t.Fatalf("Validate() error = %v, want adapter-config-invalid", err)
			}
		})
	}
}

func TestCandidateGoldenEncoding(t *testing.T) {
	t.Parallel()
	encoded, err := os.ReadFile(filepath.Join("testdata", "candidate.hex"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if got := candidateEnvelope(); !bytes.Equal(got, want) {
		t.Fatal("canonical WebTunnel envelope differs from its frozen golden bytes")
	}
}

func candidateEnvelope() []byte {
	var raw bytes.Buffer
	raw.WriteString("ardents-h3-wt1")
	raw.WriteByte(1)
	raw.WriteByte(byte(len("webtunnel-v0.0.6")))
	raw.WriteString("webtunnel-v0.0.6")
	raw.Write([]byte{203, 0, 113, 7})
	_ = binary.Write(&raw, binary.BigEndian, uint16(443))
	writeCandidateBytes(&raw, []byte("/entry"), 2)
	writeCandidateBytes(&raw, []byte("front.example"), 1)
	raw.Write(bytes.Repeat([]byte{0x5a}, 32))
	return raw.Bytes()
}

func replaceCandidatePath(raw, path []byte) []byte {
	offset := len("ardents-h3-wt1") + 1 + 1 + len("webtunnel-v0.0.6") + 4 + 2
	old := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	return replaceCandidateField(raw, offset, 2, old, path)
}

func replaceCandidateName(raw, name []byte) []byte {
	pathOffset := len("ardents-h3-wt1") + 1 + 1 + len("webtunnel-v0.0.6") + 4 + 2
	pathLength := int(binary.BigEndian.Uint16(raw[pathOffset : pathOffset+2]))
	offset := pathOffset + 2 + pathLength
	return replaceCandidateField(raw, offset, 1, int(raw[offset]), name)
}

func replaceCandidateField(raw []byte, offset, width, old int, value []byte) []byte {
	result := make([]byte, 0, len(raw)-old+len(value))
	result = append(result, raw[:offset]...)
	if width == 1 {
		result = append(result, byte(len(value)))
	} else {
		result = binary.BigEndian.AppendUint16(result, uint16(len(value)))
	}
	result = append(result, value...)
	result = append(result, raw[offset+width+old:]...)
	return result
}

func writeCandidateBytes(raw *bytes.Buffer, value []byte, width int) {
	if width == 1 {
		raw.WriteByte(byte(len(value)))
	} else {
		_ = binary.Write(raw, binary.BigEndian, uint16(len(value)))
	}
	raw.Write(value)
}
