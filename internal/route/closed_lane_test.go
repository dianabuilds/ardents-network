//go:build linux

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
	parsed, err := ReadClosedLaneFrame(bytes.NewReader(raw))
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
	decodedBootstrap, err := ReadClosedLaneFrame(bytes.NewReader(encodedBootstrap))
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
	hello := ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{7},
		Deadline: time.Unix(1_800_000_000, 0).UTC()}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameHello, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClosedLaneFrame(bytes.NewReader(raw)); err != nil {
		t.Fatalf("valid generation-3 control: %v", err)
	}
	binary.BigEndian.PutUint16(raw[4:6], 2)
	if _, err := ReadClosedLaneFrame(bytes.NewReader(raw)); err == nil {
		t.Fatal("accepted an otherwise valid generation-2 HELLO")
	}
	binary.BigEndian.PutUint16(raw[4:6], closedLaneGeneration)
	binary.BigEndian.PutUint32(raw[12:16], closedLaneMaximum+1)
	reader := &closedLaneHeaderReader{Reader: bytes.NewReader(raw[:closedLaneHeaderSize])}
	if _, err := ReadClosedLaneFrame(reader); err == nil {
		t.Fatal("accepted a body beyond the v3 maximum")
	}
	if reader.bodyRead {
		t.Fatal("attempted to read an oversized body after its header")
	}
	if _, err := DecodeClosedBootstrap([]byte{3}); err == nil {
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
func TestClosedPurposeAssignmentTablePermitsOnlyNormativeDuties(t *testing.T) {
	cases := []struct {
		purpose         ClosedPurpose
		domain, subrole uint8
		allowed         bool
	}{
		{ClosedPurposeIssuer, closedRoleDomainRendezvous, closedDutyIssuance, true},
		{ClosedPurposeName, closedRoleDomainRendezvous, closedDutyResolution, true},
		{ClosedPurposeReachability, closedRoleDomainRendezvous, closedDutyResolution, true},
		{ClosedPurposeIntroduction, closedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ClosedPurposeSubmission, closedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ClosedPurposeDataJoin, closedRoleDomainRendezvous, closedDutyDataJoin, true},
		{ClosedPurposeForwarding, closedRoleDomainInitiator, closedDutyAdjacent, true},
		{ClosedPurposeForwarding, closedRoleDomainResponder, closedDutyInterior, true},
		{ClosedPurposeIssuer, closedRoleDomainRendezvous, closedDutyResolution, false},
		{ClosedPurposeDataJoin, closedRoleDomainIntroduction, closedDutyIntroduction, false},
		{ClosedPurposeForwarding, closedRoleDomainIntroduction, closedDutyIntroduction, false},
	}
	for _, test := range cases {
		if got := ClosedPurposePermitsDuty(test.purpose, test.domain, test.subrole); got != test.allowed {
			t.Fatalf("purpose %d with %d/%d allowed=%t, want %t", test.purpose, test.domain, test.subrole, got, test.allowed)
		}
	}
}

func TestClosedLaneRejectsUnknownAndWrongLaneFormsBeforeAllocation(t *testing.T) {
	cases := []ClosedLaneFrame{
		{Kind: 13, Lane: 0},
		{Kind: closedFrameAdmit, Lane: 0, Body: make([]byte, 354)},
		{Kind: closedFrameOpen, Lane: 0, Body: make([]byte, 49)},
		{Kind: closedFrameBytes, Lane: 0, Body: []byte{1}},
		{Kind: closedFrameCredit, Lane: 1, Body: []byte{0, 0, 0, 0}},
		{Kind: closedFrameEOF, Lane: 1, Body: []byte{1}},
		{Kind: closedFrameClose, Lane: 1, Body: []byte{7}},
		{Kind: closedFrameOperation, Lane: 0, Body: make([]byte, closedSmallTerminalOperation-1)},
		{Kind: closedFrameOperation, Lane: 1, Body: make([]byte, 4095)},
		{Kind: closedFrameResult, Lane: 1, Body: make([]byte, 4096)},
		{Kind: closedFrameKeepalive, Lane: 1},
	}
	for _, frame := range cases {
		if _, err := EncodeClosedLaneFrame(frame); err == nil {
			t.Fatalf("accepted invalid closed frame: %+v", frame)
		}
	}
	for _, frame := range []ClosedLaneFrame{
		{Kind: closedFrameAdmit, Lane: 0, Body: make([]byte, 355)},
		{Kind: closedFrameOperation, Lane: 1, Body: make([]byte, closedSmallTerminalOperation)},
		{Kind: closedFrameKeepalive, Lane: 0},
	} {
		if _, err := EncodeClosedLaneFrame(frame); err != nil {
			t.Fatalf("rejected valid closed frame: %+v / %v", frame, err)
		}
	}
}
