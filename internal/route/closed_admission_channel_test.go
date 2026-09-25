package route

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

func TestClosedAdmissionChannelBindsHELLOExporterBeforeBurningToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ardp.PurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
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
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	if expected := sha256.Sum256(body); exporterContext != expected {
		t.Fatal("TLS exporter was not bound to the exact HELLO")
	}
	token := bytes.Repeat([]byte{8}, 354)
	lease, err := channel.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)})
	if err != nil || !verified || lease.Class != 2 || lease.Bytes != 32<<20 || !lease.Deadline.Equal(now.Add(time.Minute)) {
		t.Fatalf("admit lease = %+v / %v", lease, err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: make([]byte, 49)}); err == nil {
		t.Fatal("opened child without a forwarding state machine")
	}
}

// The host reservation is obtained by the credential owner before Route
// touches its spend ledger. A refusal must leave that token retryable.
func TestClosedAdmissionHostRefusalDoesNotBurnToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ardp.PurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
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
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	admit := ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{8}, 354)...)}
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

func TestClosedAdmissionCapacityRefusalRetainsHostReleaseFailure(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ardp.PurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	held := make([]*closedDutyChannel, 0, closedDutyChannels)
	for range closedDutyChannels {
		reservation, reserveErr := limits.reserveChannel()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		held = append(held, reservation)
	}
	t.Cleanup(func() {
		for _, reservation := range held {
			reservation.release()
		}
	})
	cleanup := errors.New("host release failed")
	channel, err := NewClosedAdmissionChannel(receiver, spends, limits, func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil }, func(ClosedAdmissionVerification) (ClosedAdmissionApproval, error) {
		return ClosedAdmissionApproval{Window: now.Truncate(time.Hour), Release: func() error { return cleanup }}, nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	_, err = channel.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{8}, 354)...)})
	if err == nil || !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity refusal lost primary or cleanup failure: %v", err)
	}
	if channels, children := closedAdmissionClaimCounts(limits); channels != closedDutyChannels || children != 0 {
		t.Fatalf("capacity refusal changed unrelated reservations: channels %d, children %d", channels, children)
	}
}

func TestClosedAdmissionDuplicateSpendRefusalReleasesOnlyFailedReservation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ardp.PurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	token := bytes.Repeat([]byte{8}, 354)
	open := func(release func() error) *ClosedAdmissionChannel {
		channel, openErr := NewClosedAdmissionChannel(receiver, spends, limits, func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil }, func(ClosedAdmissionVerification) (ClosedAdmissionApproval, error) {
			return ClosedAdmissionApproval{Window: now.Truncate(time.Hour), Release: release}, nil
		}, func() time.Time { return now })
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, openErr = channel.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); openErr != nil {
			t.Fatal(openErr)
		}
		return channel
	}
	first := open(nil)
	lease, err := first.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)})
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.Release(); err != nil {
		t.Fatal(err)
	}
	cleanup := errors.New("host release failed")
	var released int
	keeper := open(nil)
	keeperLease, err := keeper.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{6}, 354)...)})
	if err != nil {
		t.Fatal(err)
	}
	second := open(func() error { released++; return cleanup })
	_, err = second.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)})
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "token") || released != 1 {
		t.Fatalf("duplicate refusal = %v / releases %d", err, released)
	}
	if channels, children := closedAdmissionClaimCounts(limits); channels != 1 || children != 0 {
		t.Fatalf("duplicate refusal changed live reservation: channels %d, children %d", channels, children)
	}
	third := open(func() error { released++; return nil })
	other := bytes.Repeat([]byte{7}, 354)
	lease, err = third.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, other...)})
	if err != nil {
		t.Fatalf("failed duplicate cleanup retained unrelated capacity: %v", err)
	}
	if err = lease.Release(); err != nil || released != 2 {
		t.Fatalf("healthy release = %v / %d", err, released)
	}
	if channels, children := closedAdmissionClaimCounts(limits); channels != 1 || children != 0 {
		t.Fatalf("healthy cleanup changed live reservation: channels %d, children %d", channels, children)
	}
	if err = keeperLease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedAdmissionExpiredLeaseRefusalReleasesOnlyFailedReservation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := now
	receiver := ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6, RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, ExpectedPurpose: ardp.PurposeIssuer, NotAfter: now.Add(time.Hour)}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := NewClosedDutyLimits(func() time.Time { return clock })
	if err != nil {
		t.Fatal(err)
	}
	cleanup := errors.New("host release failed")
	released := 0
	keeper, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.release)
	channel, err := NewClosedAdmissionChannel(receiver, spends, limits, func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil }, func(ClosedAdmissionVerification) (ClosedAdmissionApproval, error) {
		clock = now.Add(time.Minute)
		return ClosedAdmissionApproval{Window: now.Truncate(time.Hour), Release: func() error { released++; return cleanup }}, nil
	}, func() time.Time { return clock })
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, ChannelNonce: [32]byte{7}, Deadline: now.Add(time.Minute)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	_, err = channel.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{8}, 354)...)})
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "lease") || released != 1 {
		t.Fatalf("expired refusal = %v / releases %d", err, released)
	}
	if channels, children := closedAdmissionClaimCounts(limits); channels != 1 || children != 0 {
		t.Fatalf("expired refusal changed live reservation: channels %d, children %d", channels, children)
	}
}
