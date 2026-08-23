package record

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"
)

func TestRecordSignatureBindsDomainLiteral(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	record := []byte("canonical-record")
	signature := ed25519.Sign(private, recordTranscript(network, record))
	wrong := transcriptForTest("ardents-name-record-v2", network, record)
	if ed25519.Verify(public, wrong, signature) {
		t.Fatal("record signature was valid under another domain literal")
	}
}

func transcriptForTest(domain string, network [32]byte, record []byte) []byte {
	out := binary.BigEndian.AppendUint16(nil, uint16(len(domain)))
	out = append(out, domain...)
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(len(record)))
	return append(out, record...)
}
