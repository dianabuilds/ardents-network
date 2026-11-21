package wire

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"reflect"
	"testing"
)

// independentEnvelope spells the accepted wire contract without the encoder's
// constants or payload builders. These fixtures confer no stream authority.
func independentEnvelope(kind byte, payload []byte) []byte {
	body := []byte{0, 3, kind, 37}
	body = append(body, "ardents-service-connection-profile-v3"...)
	body = append(body, payload...)
	wire := []byte("ardents-service-connection-v3\x00")
	wire = binary.BigEndian.AppendUint16(wire, uint16(len(body)))
	return append(wire, body...)
}

func independentOffsets(generation, offset uint64) []byte {
	b := binary.BigEndian.AppendUint64(nil, generation)
	return binary.BigEndian.AppendUint64(b, offset)
}

func TestRecordIndependentEncodings(t *testing.T) {
	t.Parallel()
	challenge := Challenge{Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 3,
		Context: [32]byte{4}, Nonce: [32]byte{5}}
	challengeBytes := append(append([]byte(nil), challenge.Network[:]...), challenge.Target[:]...)
	challengeBytes = binary.BigEndian.AppendUint64(challengeBytes, 3)
	challengeBytes = append(challengeBytes, challenge.Context[:]...)
	challengeBytes = append(challengeBytes, challenge.Nonce[:]...)
	proof := Proof{ChallengeDigest: sha256.Sum256(independentEnvelope(1, challengeBytes)), Signature: [64]byte{6}}
	proofBytes := append(append([]byte(nil), proof.ChallengeDigest[:]...), proof.Signature[:]...)
	dataBytes := append(independentOffsets(7, 8), 0, 2, 9, 10)
	cases := []struct {
		name   string
		record Record
		wire   []byte
	}{
		{"challenge", Record{Challenge: &challenge}, independentEnvelope(1, challengeBytes)},
		{"proof", Record{Proof: &proof}, independentEnvelope(2, proofBytes)},
		{"data", Record{Data: &Data{7, 8, []byte{9, 10}}}, independentEnvelope(4, dataBytes)},
		{"ack", Record{Acknowledgement: &Acknowledgement{7, 10, false, false}}, independentEnvelope(5, independentOffsets(7, 10))},
		{"receipt", Record{Acknowledgement: &Acknowledgement{7, 10, true, false}}, independentEnvelope(5, append(independentOffsets(7, 10), 1))},
		{"confirmation", Record{Acknowledgement: &Acknowledgement{7, 10, true, true}}, independentEnvelope(5, append(independentOffsets(7, 10), 2))},
		{"terminal", Record{Terminal: &Terminal{7, 10}}, independentEnvelope(6, independentOffsets(7, 10))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := Write(&encoded, tc.record); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded.Bytes(), tc.wire) {
				t.Fatalf("wire differs: %x vs %x", encoded.Bytes(), tc.wire)
			}
			decoded, err := Read(bytes.NewReader(tc.wire))
			if err != nil || !reflect.DeepEqual(decoded, tc.record) {
				t.Fatalf("decoded=%+v error=%v", decoded, err)
			}
		})
	}
	digest, err := ChallengeDigest(challenge)
	if err != nil || digest != proof.ChallengeDigest {
		t.Fatalf("challenge transcript differs: %x %v", digest, err)
	}
}

func TestContinuityIndependentTranscript(t *testing.T) {
	t.Parallel()
	key, context, exporter := [32]byte{1}, [32]byte{2}, [32]byte{3}
	nonceMAC := hmac.New(sha256.New, key[:])
	nonceMAC.Write([]byte("ardents-service-connection-nonce-v3\x00"))
	nonceMAC.Write([]byte{1})
	nonceMAC.Write(binary.BigEndian.AppendUint64(nil, 4))
	nonceMAC.Write(exporter[:])
	transcript := []byte{1}
	for _, n := range []uint64{4, 5, 6, 7} {
		transcript = binary.BigEndian.AppendUint64(transcript, n)
	}
	transcript = append(transcript, nonceMAC.Sum(nil)...)
	transcript = append(transcript, context[:]...)
	transcript = append(transcript, exporter[:]...)
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte("ardents-service-connection-continuity-v3\x00"))
	mac.Write(transcript)
	independent := independentEnvelope(3, append(transcript, mac.Sum(nil)...))
	c, err := NewContinuity(key, RoleInitiator, 4, 5, 6, 7, context, exporter)
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := Write(&wire, Record{Continuity: &c}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire.Bytes(), independent) {
		t.Fatal("continuity encoding/domain differs")
	}
	if err := VerifyContinuity(key, c, RoleInitiator, 4, context, exporter); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Continuity){
		func(c *Continuity) { c.Role = RolePublisher }, func(c *Continuity) { c.AttachmentGeneration++ },
		func(c *Continuity) { c.SendBase++ }, func(c *Continuity) { c.SendEnd++ }, func(c *Continuity) { c.ReceiveNext++ },
		func(c *Continuity) { c.Nonce[0]++ }, func(c *Continuity) { c.Context[0]++ },
		func(c *Continuity) { c.ExporterCommitment[0]++ }, func(c *Continuity) { c.MAC[0]++ },
	} {
		changed := c
		mutate(&changed)
		if VerifyContinuity(key, changed, RoleInitiator, 4, context, exporter) == nil {
			t.Fatal("altered authenticated field accepted")
		}
	}
	if _, err := NewContinuity([32]byte{}, RoleInitiator, 1, 0, 0, 0, context, exporter); err == nil {
		t.Fatal("absent continuity secret accepted")
	}
	if VerifyContinuity(key, c, 0, 4, context, exporter) == nil {
		t.Fatal("unknown expected role accepted")
	}
}

func TestRecordRefusesMalformedAndTruncatedInputs(t *testing.T) {
	t.Parallel()
	valid := independentEnvelope(4, append(independentOffsets(1, 0), 0, 1, 42))
	for end := 0; end < len(valid); end++ {
		if _, err := Read(bytes.NewReader(valid[:end])); err == nil || errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("truncation at %d was accepted or classified as a complete contradiction: %v", end, err)
		}
	}
	bodyAt := len("ardents-service-connection-v3\x00") + 2
	for _, at := range []int{0, bodyAt, bodyAt + 1, bodyAt + 2, bodyAt + 3, bodyAt + 4, len(valid) - 2} {
		changed := append([]byte(nil), valid...)
		changed[at] ^= 127
		if _, err := Read(bytes.NewReader(changed)); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("complete mutation at %d was not an integrity violation: %v", at, err)
		}
	}
	for _, wire := range [][]byte{
		independentEnvelope(6, append(independentOffsets(1, 0), 0)),
		independentEnvelope(5, append(independentOffsets(1, 0), 0)),
		independentEnvelope(5, append(independentOffsets(1, 0), 3)),
		independentEnvelope(4, append(independentOffsets(1, math.MaxUint64), 0, 1, 42)),
		independentEnvelope(4, append(independentOffsets(1, 0), 0, 0)),
		independentEnvelope(4, append(append(independentOffsets(1, 0), 0x40, 1), make([]byte, 16385)...)),
		independentEnvelope(6, independentOffsets(0, 0)),
	} {
		if _, err := Read(bytes.NewReader(wire)); !errors.Is(err, ErrInvalidRecord) {
			t.Fatal("invalid length/range/marker was not an integrity violation", err)
		}
	}
	if _, err := Read(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	old := append([]byte(nil), valid...)
	old[len("ardents-service-connection-v")] = '2'
	if _, err := Read(bytes.NewReader(old)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("old record was not an integrity violation", err)
	}
}

type failedRecordReader struct{ cause error }

func (r failedRecordReader) Read([]byte) (int, error) { return 0, r.cause }

func TestRecordRetainsPhysicalFailureAtEveryReadBoundary(t *testing.T) {
	t.Parallel()
	valid := independentEnvelope(4, append(independentOffsets(1, 0), 0, 1, 42))
	cause := errors.New("original physical read failure")
	for end := 0; end < len(valid); end++ {
		reader := io.MultiReader(bytes.NewReader(valid[:end]), failedRecordReader{cause})
		if _, err := ReadStream(reader); !errors.Is(err, cause) || errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("physical failure at %d lost its cause or became an integrity violation: %v", end, err)
		}
	}
	// A complete contradiction is already observable before the failed next
	// physical read. Its rejection must not wait for more bytes or be hidden by
	// that later failure.
	badPrefix := []byte("ardents-service-connection-v2\x00")
	reader := io.MultiReader(bytes.NewReader(badPrefix), failedRecordReader{cause})
	if _, err := Read(reader); !errors.Is(err, ErrInvalidRecord) || errors.Is(err, cause) {
		t.Fatalf("complete invalid prefix was masked by a later reader failure: %v", err)
	}
}

type shortRecordWriter struct {
	calls   int
	failure error
}

func (w *shortRecordWriter) Write(p []byte) (int, error) { w.calls++; return len(p) - 1, w.failure }

func TestRecordWriteRefusesBeforeEffectsAndRetainsShortWrite(t *testing.T) {
	w := &shortRecordWriter{}
	for _, r := range []Record{
		{}, {Terminal: &Terminal{1, 0}, Data: &Data{1, 0, []byte{1}}},
		{Data: &Data{1, math.MaxUint64, []byte{1}}},
		{Acknowledgement: &Acknowledgement{1, 0, false, true}},
	} {
		if err := Write(w, r); err == nil {
			t.Fatal("invalid record accepted")
		}
	}
	if w.calls != 0 {
		t.Fatal("invalid record emitted bytes")
	}
	r := Record{Terminal: &Terminal{1, 0}}
	if err := Write(w, r); !errors.Is(err, io.ErrShortWrite) || w.calls != 1 {
		t.Fatalf("short write=%v calls=%d", err, w.calls)
	}
	w.failure = errors.New("physical output failed")
	if err := Write(w, r); err != w.failure {
		t.Fatalf("physical failure replaced: %v", err)
	}
	if err := Write(nil, r); err == nil {
		t.Fatal("nil writer accepted")
	}
}

func TestStreamRecordRefusesAuthenticationAndPreservesFraming(t *testing.T) {
	c := Challenge{Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 3, Context: [32]byte{4}, Nonce: [32]byte{5}}
	var wire bytes.Buffer
	if err := Write(&wire, Record{Challenge: &c}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStream(&wire); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("challenge entered Application direction or lost its integrity classification", err)
	}
	first := independentEnvelope(4, append(independentOffsets(1, 0), 0, 1, 42))
	second := independentEnvelope(6, independentOffsets(1, 1))
	reader := bytes.NewReader(append(first, second...))
	if r, err := ReadStream(reader); err != nil || r.Data == nil || r.Data.Payload[0] != 42 {
		t.Fatal("data framing differs")
	}
	if r, err := ReadStream(reader); err != nil || r.Terminal == nil || r.Terminal.Offset != 1 || reader.Len() != 0 {
		t.Fatal("terminal framing differs")
	}
}
