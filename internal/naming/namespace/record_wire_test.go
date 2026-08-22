package namespace

import (
	"bytes"
	"strings"
	"testing"
)

func TestRecordEncodingIsDeterministicAndRoundTrips(t *testing.T) {
	t.Parallel()
	record := claimRoot(t, "site", "alice", 100)
	first, err := EncodeRecord(record)
	if err != nil {
		t.Fatalf("EncodeRecord: %v", err)
	}
	second, err := EncodeRecord(record)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("record encoding is not deterministic: %v", err)
	}
	decoded, err := DecodeRecord(first)
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if decoded != record {
		t.Fatalf("round trip changed record: got=%+v want=%+v", decoded, record)
	}
}

func TestRecordEncoderRejectsUnknownStateAndPreservesOpaqueField(t *testing.T) {
	t.Parallel()
	record := claimRoot(t, "site", "alice", 100)
	record.Consistency = "guessed"
	if _, err := EncodeRecord(record); err == nil {
		t.Fatal("EncodeRecord accepted an unknown consistency state")
	}
	record.Consistency = consistencyCurrent
	record.Authority = strings.Repeat("x", 5000)
	wire, err := EncodeRecord(record)
	if err != nil {
		t.Fatalf("EncodeRecord rejected an opaque field without a selected bound: %v", err)
	}
	decoded, err := DecodeRecord(wire)
	if err != nil || decoded.Authority != record.Authority {
		t.Fatalf("opaque field round trip failed: %v", err)
	}
}

func TestRecordDecoderRejectsMutationAndTrailingBytes(t *testing.T) {
	t.Parallel()
	record := claimRoot(t, "site", "alice", 100)
	wire, err := EncodeRecord(record)
	if err != nil {
		t.Fatalf("EncodeRecord: %v", err)
	}
	for _, mutated := range [][]byte{
		append(append([]byte(nil), wire...), 0),
		append([]byte{0, 5}, wire[2:]...),
		wire[:len(wire)-1],
	} {
		if _, err := DecodeRecord(mutated); err == nil {
			t.Fatalf("DecodeRecord accepted mutation %x", mutated)
		}
	}
}

func TestApplyRejectsNonCanonicalNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{" Site.Example", "Site.Example", "site.example ", "site..example"} {
		if _, err := Apply(nil, 100, Op{Kind: "claim", Name: name, Generation: 1,
			Authority: "alice"}, testPolicy); err == nil {
			t.Fatalf("Apply accepted non-canonical name %q", name)
		}
	}
}
