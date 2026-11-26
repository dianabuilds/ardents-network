package ardp

import (
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCanonicalFrameBytes(t *testing.T) {
	// Literal oracle: ARDP, generation 3, CREDIT, no flags, lane 1,
	// length 4, credit 65536. It is independent of EncodeFrame's builder.
	oracle, err := hex.DecodeString("4152445000030700000000010000000400010000")
	if err != nil {
		t.Fatal(err)
	}
	frame := Frame{Kind: KindCredit, Lane: 1, Body: []byte{0, 1, 0, 0}}
	raw, err := EncodeFrame(frame)
	if err != nil || !bytes.Equal(raw, oracle) {
		t.Fatalf("canonical frame: %x, %v", raw, err)
	}
	decoded, err := ReadFrame(bytes.NewReader(oracle))
	if err != nil || decoded.Kind != frame.Kind || decoded.Lane != 1 || !bytes.Equal(decoded.Body, frame.Body) {
		t.Fatalf("decode: %+v %v", decoded, err)
	}
}

func TestBootstrapOperationVocabulary(t *testing.T) {
	// Independent complete wire oracle: generation 3, BOOTSTRAP, lane zero,
	// exactly one operation byte. Parsing grants no bootstrap authority.
	header, err := hex.DecodeString("41524450000303000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	for operation := 0; operation <= 255; operation++ {
		body := []byte{byte(operation)}
		wire := append(append([]byte(nil), header...), body...)
		allowed := operation == 1 || operation == 2
		decoded, readErr := ReadFrame(bytes.NewReader(wire))
		if allowed {
			if readErr != nil || decoded.Kind != KindBootstrap || decoded.Lane != 0 || !bytes.Equal(decoded.Body, body) {
				t.Fatalf("operation %d decode: %+v, %v", operation, decoded, readErr)
			}
		} else if readErr == nil {
			t.Fatalf("unknown operation %d decoded", operation)
		}
		encoded, encodeErr := EncodeFrame(Frame{Kind: KindBootstrap, Body: body})
		if allowed {
			if encodeErr != nil || !bytes.Equal(encoded, wire) {
				t.Fatalf("operation %d encode: %x, %v", operation, encoded, encodeErr)
			}
		} else if encodeErr == nil {
			t.Fatalf("unknown operation %d encoded", operation)
		}
	}
}

func TestCanonicalHelloAndAcceptBytes(t *testing.T) {
	// Independent fixed layout: five identities, duty 1, forwarding Purpose,
	// nonce, and Unix second 1. No production encoder constructs this oracle.
	oracle, err := hex.DecodeString(strings.Repeat("01", 160) + "000000000000000107" + strings.Repeat("02", 32) + "0000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	hello, err := DecodeHello(oracle)
	if err != nil || hello.Purpose != PurposeForwarding || hello.RecipientDutyGeneration != 1 || !hello.Deadline.Equal(time.Unix(1, 0)) {
		t.Fatalf("fixed HELLO decode: %+v %v", hello, err)
	}
	encoded, err := EncodeHello(hello)
	if err != nil || !bytes.Equal(encoded, oracle) {
		t.Fatalf("fixed HELLO encode: %x %v", encoded, err)
	}
	hello.Purpose = 8
	if _, err := EncodeHello(hello); err == nil {
		t.Fatal("unknown Purpose accepted")
	}
	for status := uint8(0); status <= 4; status++ {
		credit := uint32(0)
		if status == 0 {
			credit = 65536
		}
		frame, err := AcceptFrame(status, credit)
		if err != nil {
			t.Fatal(err)
		}
		var written bytes.Buffer
		if err := WriteFrame(&written, frame); err != nil {
			t.Fatal(err)
		}
		decoded, err := ReadFrame(&written)
		if err != nil {
			t.Fatal(err)
		}
		gotStatus, gotCredit, err := DecodeAcceptFrame(decoded)
		if err != nil || gotStatus != status || gotCredit != credit {
			t.Fatalf("acceptance: %d %d %v", gotStatus, gotCredit, err)
		}
	}
	if _, err := AcceptFrame(1, 1); err == nil {
		t.Fatal("refusal carried credit")
	}
}

type headerOnlyReader struct {
	header   []byte
	bodyRead bool
}

func (reader *headerOnlyReader) Read(target []byte) (int, error) {
	if len(reader.header) == 0 {
		reader.bodyRead = true
		return 0, io.EOF
	}
	n := copy(target, reader.header)
	reader.header = reader.header[n:]
	return n, nil
}

func TestRejectHeaderBeforeReadingBody(t *testing.T) {
	for _, raw := range []string{
		"415244500003ff000000000000004000", // unknown kind
		"41524450000301000000000000004000", // oversized HELLO
		"41524450000302000000000100000163", // child ADMIT
		"41524450000308000000000000000000", // lane-zero EOF
		"41524450000306010000000100000001", // flags
	} {
		t.Run(raw, func(t *testing.T) {
			header, err := hex.DecodeString(raw)
			if err != nil {
				t.Fatal(err)
			}
			reader := &headerOnlyReader{header: header}
			if _, err := ReadFrame(reader); err == nil || reader.bodyRead {
				t.Fatalf("header refusal after body read=%v, err=%v", reader.bodyRead, err)
			}
		})
	}
}

func TestCreditAndAdmissionRefusal(t *testing.T) {
	for _, frame := range []Frame{
		{Kind: KindCredit, Lane: 1, Body: []byte{0, 0, 0, 0}},
		{Kind: KindCredit, Lane: 1, Body: []byte{0, 1, 0, 1}},
		{Kind: KindAdmit, Lane: 1, Body: make([]byte, 355)},
		{Kind: KindClose, Lane: 0, Body: []byte{7}},
		{Kind: KindClose, Lane: 0, Body: []byte{0}},
	} {
		if _, err := EncodeFrame(frame); err == nil {
			t.Fatalf("invalid frame accepted: %+v", frame)
		}
	}
	if _, err := EncodeFrame(Frame{Kind: KindClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatalf("successful child CLOSE refused: %v", err)
	}
}

func TestPartialFrameIsNotDecoded(t *testing.T) {
	raw, err := hex.DecodeString("415244500003060000000001000000036162")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(bytes.NewReader(raw)); err != io.ErrUnexpectedEOF {
		t.Fatalf("partial frame: %v", err)
	}
}
