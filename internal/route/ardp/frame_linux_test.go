//go:build linux

package ardp

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func TestClosedLaneHELLOAndBootstrapHaveExactV3Framing(t *testing.T) {
	hello := Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: PurposeForwarding, ChannelNonce: [32]byte{7},
		Deadline: time.Unix(1_800_000_000, 0).UTC()}
	body, err := EncodeHello(hello)
	if err != nil || len(body) != 209 {
		t.Fatalf("encode HELLO = %d / %v", len(body), err)
	}
	frame := Frame{Kind: KindHello, Lane: 0, Body: body}
	raw, err := EncodeFrame(frame)
	if err != nil || len(raw) != HeaderSize+209 || string(raw[:4]) != closedLaneMagic || binary.BigEndian.Uint16(raw[4:6]) != closedLaneGeneration {
		t.Fatalf("encode lane frame = %x / %v", raw, err)
	}
	parsed, err := ReadFrame(bytes.NewReader(raw))
	decoded, helloErr := DecodeHello(parsed.Body)
	if err != nil || helloErr != nil || decoded != hello {
		t.Fatalf("decode HELLO = %+v / %v / %v", decoded, err, helloErr)
	}
	if read, err := ReadFrame(bytes.NewReader(raw)); err != nil || read.Kind != KindHello || read.Lane != 0 {
		t.Fatalf("stream read = %+v / %v", read, err)
	}
	bootstrap := Frame{Kind: KindBootstrap, Lane: 0, Body: EncodeBootstrap(true)}
	encodedBootstrap, err := EncodeFrame(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	decodedBootstrap, err := ReadFrame(bytes.NewReader(encodedBootstrap))
	issuer, operationErr := DecodeBootstrap(decodedBootstrap.Body)
	if err != nil || operationErr != nil || !issuer {
		t.Fatalf("bootstrap = %+v / %v / %v", decodedBootstrap, err, operationErr)
	}
	accepted, err := AcceptFrame(0, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	status, credit, err := DecodeAcceptFrame(accepted)
	if err != nil || status != 0 || credit != 64<<10 {
		t.Fatalf("accept = %d / %d / %v", status, credit, err)
	}
}

func TestClosedLaneRejectsGenerationTwoAndOversizedAllocation(t *testing.T) {
	hello := Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: PurposeForwarding, ChannelNonce: [32]byte{7},
		Deadline: time.Unix(1_800_000_000, 0).UTC()}
	body, err := EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeFrame(Frame{Kind: KindHello, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(bytes.NewReader(raw)); err != nil {
		t.Fatalf("valid generation-3 control: %v", err)
	}
	binary.BigEndian.PutUint16(raw[4:6], 2)
	if _, err := ReadFrame(bytes.NewReader(raw)); err == nil {
		t.Fatal("accepted an otherwise valid generation-2 HELLO")
	}
	binary.BigEndian.PutUint16(raw[4:6], closedLaneGeneration)
	binary.BigEndian.PutUint32(raw[12:16], MaximumBodySize+1)
	reader := &closedLaneHeaderReader{Reader: bytes.NewReader(raw[:HeaderSize])}
	if _, err := ReadFrame(reader); err == nil {
		t.Fatal("accepted a body beyond the v3 maximum")
	}
	if reader.bodyRead {
		t.Fatal("attempted to read an oversized body after its header")
	}
	if _, err := DecodeBootstrap([]byte{3}); err == nil {
		t.Fatal("accepted an unknown bootstrap operation")
	}
}

// closedLaneHeaderReader records attempts to consume a rejected frame's body.
type closedLaneHeaderReader struct {
	*bytes.Reader
	bodyRead bool
}

func (reader *closedLaneHeaderReader) Read(p []byte) (int, error) {
	if reader.Len() == 0 && len(p) > 0 {
		reader.bodyRead = true
	}
	return reader.Reader.Read(p)
}
func TestClosedLaneRejectsUnknownAndWrongLaneFormsBeforeAllocation(t *testing.T) {
	cases := []Frame{
		{Kind: 13, Lane: 0},
		{Kind: KindAdmit, Lane: 0, Body: make([]byte, 354)},
		{Kind: KindOpen, Lane: 0, Body: make([]byte, 49)},
		{Kind: KindBytes, Lane: 0, Body: []byte{1}},
		{Kind: KindCredit, Lane: 1, Body: []byte{0, 0, 0, 0}},
		{Kind: KindEOF, Lane: 1, Body: []byte{1}},
		{Kind: KindClose, Lane: 1, Body: []byte{7}},
		{Kind: KindOperation, Lane: 0, Body: make([]byte, terminal.SmallBodySize-1)},
		{Kind: KindOperation, Lane: 1, Body: make([]byte, 4095)},
		{Kind: KindResult, Lane: 1, Body: make([]byte, 4096)},
		{Kind: KindKeepalive, Lane: 1},
	}
	for _, frame := range cases {
		if _, err := EncodeFrame(frame); err == nil {
			t.Fatalf("accepted invalid closed frame: %+v", frame)
		}
	}
	for _, frame := range []Frame{
		{Kind: KindAdmit, Lane: 0, Body: make([]byte, 355)},
		{Kind: KindOperation, Lane: 1, Body: make([]byte, terminal.SmallBodySize)},
		{Kind: KindKeepalive, Lane: 0},
	} {
		if _, err := EncodeFrame(frame); err != nil {
			t.Fatalf("rejected valid closed frame: %+v / %v", frame, err)
		}
	}
}
