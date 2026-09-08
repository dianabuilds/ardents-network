package route

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestClosedLaneHELLOAndBootstrapHaveExactV3Framing(t *testing.T) {
	hello := ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{7},
		Deadline: time.Unix(1_800_000_000, 0).UTC()}
	body, err := EncodeClosedHello(hello)
	if err != nil || len(body) != 209 {
		t.Fatalf("encode HELLO = %d / %v", len(body), err)
	}
	frame := ClosedLaneFrame{Kind: closedFrameHello, Lane: 0, Body: body}
	raw, err := EncodeClosedLaneFrame(frame)
	if err != nil || len(raw) != closedLaneHeaderSize+209 || string(raw[:4]) != closedLaneMagic || binary.BigEndian.Uint16(raw[4:6]) != closedLaneGeneration {
		t.Fatalf("encode lane frame = %x / %v", raw, err)
	}
	parsed, err := DecodeClosedLaneFrame(raw)
	decoded, helloErr := DecodeClosedHello(parsed.Body)
	if err != nil || helloErr != nil || decoded != hello {
		t.Fatalf("decode HELLO = %+v / %v / %v", decoded, err, helloErr)
	}
	if read, err := ReadClosedLaneFrame(bytes.NewReader(raw)); err != nil || read.Kind != closedFrameHello || read.Lane != 0 {
		t.Fatalf("stream read = %+v / %v", read, err)
	}
	bootstrap := ClosedLaneFrame{Kind: closedFrameBootstrap, Lane: 0, Body: EncodeClosedBootstrap(true)}
	encodedBootstrap, err := EncodeClosedLaneFrame(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	decodedBootstrap, err := DecodeClosedLaneFrame(encodedBootstrap)
	issuer, operationErr := DecodeClosedBootstrap(decodedBootstrap.Body)
	if err != nil || operationErr != nil || !issuer {
		t.Fatalf("bootstrap = %+v / %v / %v", decodedBootstrap, err, operationErr)
	}
	accepted, err := ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	status, credit, err := DecodeClosedAcceptFrame(accepted)
	if err != nil || status != 0 || credit != 64<<10 {
		t.Fatalf("accept = %d / %d / %v", status, credit, err)
	}
}

func TestClosedLaneRejectsGenerationTwoAndOversizedAllocation(t *testing.T) {
	raw := make([]byte, closedLaneHeaderSize)
	copy(raw[:4], closedLaneMagic)
	binary.BigEndian.PutUint16(raw[4:6], 2)
	raw[6] = closedFrameHello
	binary.BigEndian.PutUint32(raw[12:16], 209)
	if _, err := DecodeClosedLaneFrame(raw); err == nil {
		t.Fatal("accepted a generation-2 lane header")
	}
	binary.BigEndian.PutUint16(raw[4:6], closedLaneGeneration)
	binary.BigEndian.PutUint32(raw[12:16], closedLaneMaximum+1)
	if _, err := ReadClosedLaneFrame(bytes.NewReader(raw)); err == nil {
		t.Fatal("allocated a body beyond the v3 maximum")
	}
	if _, err := DecodeClosedBootstrap([]byte{3}); err == nil {
		t.Fatal("accepted an unknown bootstrap operation")
	}
}
