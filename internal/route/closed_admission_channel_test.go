package route

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestClosedAdmissionChannelBindsHELLOExporterBeforeBurningToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ClosedPurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = spends.Close() }()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	var exported, verified bool
	var exporterContext [32]byte
	channel, err := NewClosedAdmissionChannel(receiver, spends, limits, func(label string, context []byte, length int) ([]byte, error) {
		if label != closedChannelExporterLabel || length != 32 || len(context) != 32 {
			t.Fatalf("exporter input = %q / %d / %d", label, length, len(context))
		}
		copy(exporterContext[:], context)
		exported = true
		return bytes.Repeat([]byte{9}, 32), nil
	}, func(input ClosedAdmissionVerification) (ClosedAdmissionApproval, error) {
		verified = true
		if !exported || input.Class != 2 || input.Exporter != [32]byte(bytes.Repeat([]byte{9}, 32)) {
			t.Fatal("token verification lacked TLS binding")
		}
		return ClosedAdmissionApproval{Window: now.Truncate(time.Hour)}, nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	if expected := sha256.Sum256(body); exporterContext != expected {
		t.Fatal("TLS exporter was not bound to the exact HELLO")
	}
	token := bytes.Repeat([]byte{8}, 354)
	lease, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameAdmit, Body: append([]byte{2}, token...)})
	if err != nil || !verified || lease.Class != 2 || lease.Bytes != 32<<20 || !lease.Deadline.Equal(now.Add(time.Minute)) {
		t.Fatalf("admit lease = %+v / %v", lease, err)
	}
	if _, err = channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: make([]byte, 49)}); err == nil {
		t.Fatal("opened child without a forwarding state machine")
	}
}

// The host reservation is obtained by the credential owner before Route
// touches its spend ledger. A refusal must leave that token retryable.
func TestClosedAdmissionHostRefusalDoesNotBurnToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ClosedPurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	available, released := false, 0
	channel, err := NewClosedAdmissionChannel(receiver, spends, limits,
		func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil },
		func(input ClosedAdmissionVerification) (ClosedAdmissionApproval, error) {
			if input.Class != 2 || input.Deadline != now.Add(time.Minute) {
				t.Fatalf("host reservation input differs: %+v", input)
			}
			if !available {
				return ClosedAdmissionApproval{}, errors.New("host reserve refused")
			}
			return ClosedAdmissionApproval{Window: now.Truncate(time.Hour), Release: func() error { released++; return nil }}, nil
		}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	admit := ClosedLaneFrame{Kind: closedFrameAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{8}, 354)...)}
	if _, err = channel.Accept(admit); err == nil {
		t.Fatal("host refusal admitted or spent the token")
	}
	available = true
	lease, err := channel.Accept(admit)
	if err != nil {
		t.Fatalf("same token was not retryable after host refusal: %v", err)
	}
	if err := lease.Release(); err != nil || released != 1 {
		t.Fatalf("host reservation release = %v / %d", err, released)
	}
}
