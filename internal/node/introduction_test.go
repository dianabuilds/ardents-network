package node

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestIntroductionForwardsOneSealedRecordThenReportsUnavailable(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 51, "introduction")
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	network, digest, nodeID := [32]byte{1}, [32]byte{2}, [32]byte{3}
	reachability, join := [32]byte{4}, [32]byte{5}
	running, err := StartIntroduction(IntroductionConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: nodeID, NodePublicKey: public, Epoch: 6, NotAfter: deadline,
		Admit: introductionTestAdmit(network, digest, nodeID, deadline), HandshakeLimit: 2, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	publisherAuthorization := []byte("publisher-capability")
	publisher := openIntroductionAttachment(t, running.listener.Addr().String(), public, network, digest, nodeID, 6, deadline,
		[32]byte{7}, publisherAuthorization)
	defer publisher.Close()
	registration := route.IntroductionSlotRegistration{Reachability: reachability, JoinHandle: join, NotAfter: deadline}
	if err := route.WriteIntroductionSlotRegistration(publisher, registration); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadIntroductionSlotReady(publisher)
	if err != nil || ready.Reachability != reachability || ready.JoinHandle != join || !ready.NotAfter.Equal(deadline) {
		t.Fatalf("slot ready = %+v, %v", ready, err)
	}
	sealed := route.SealedIntroduction{NetworkID: network, Digest: digest, Epoch: 6, IntroductionNodeID: nodeID, RendezvousNodeID: [32]byte{8},
		Reachability: reachability, NotAfter: deadline, JoinHandle: join, EndpointHandshake: [32]byte{9}, Enc: bytes.Repeat([]byte{1}, 32), Ciphertext: bytes.Repeat([]byte{2}, 16)}
	first := [32]byte{10}
	userAuthorization := []byte("State Transit Grant distinct from JoinHandle")
	if result := submitIntroduction(t, running.listener.Addr().String(), public, network, digest, nodeID, deadline, first, userAuthorization, sealed); result.Outcome != route.IntroductionDelivered {
		t.Fatalf("first delivery = %+v", result)
	}
	received, err := route.ReadIntroductionControlRecord(publisher)
	if err != nil || received.Sealed == nil || received.Sealed.NetworkID != sealed.NetworkID || received.Sealed.Digest != sealed.Digest ||
		received.Sealed.EndpointHandshake != sealed.EndpointHandshake || !bytes.Equal(received.Sealed.Enc, sealed.Enc) || !bytes.Equal(received.Sealed.Ciphertext, sealed.Ciphertext) {
		t.Fatalf("Publisher sealed delivery = %+v, %v", received, err)
	}
	second := [32]byte{11}
	if result := submitIntroduction(t, running.listener.Addr().String(), public, network, digest, nodeID, deadline, second, userAuthorization, sealed); result.Outcome != route.IntroductionUnavailable {
		t.Fatalf("spent JoinHandle result = %+v", result)
	}
	awaitIntroductionUsage(t, running, time.Second, func(usage IntroductionUsage) bool {
		return usage.Slots == 0 && usage.Delivered == 1 && usage.Unavailable >= 1
	})
}

func TestIntroductionSlotExpiresAtItsRegistrationDeadline(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 54, "introduction-slot-expiry")
	now := time.Now().UTC().Truncate(time.Second)
	nodeDeadline := now.Add(10 * time.Second)
	slotDeadline := now.Add(3 * time.Second)
	network, digest, nodeID := [32]byte{14}, [32]byte{15}, [32]byte{16}
	reachability, join := [32]byte{17}, [32]byte{18}
	running, err := StartIntroduction(IntroductionConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: nodeID, NodePublicKey: public, Epoch: 6, NotAfter: nodeDeadline,
		Admit: introductionTestAdmit(network, digest, nodeID, slotDeadline), HandshakeLimit: 1, SlotLimit: 1, DeliveryLimit: 1,
		AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	publisher := openIntroductionAttachment(t, running.listener.Addr().String(), public, network, digest, nodeID, 6, slotDeadline,
		[32]byte{20}, []byte("publisher-slot-expiry"))
	defer publisher.Close()
	if err := route.WriteIntroductionSlotRegistration(publisher, route.IntroductionSlotRegistration{Reachability: reachability, JoinHandle: join, NotAfter: slotDeadline}); err != nil {
		t.Fatal(err)
	}
	if ready, err := route.ReadIntroductionSlotReady(publisher); err != nil || ready.NotAfter != slotDeadline {
		t.Fatalf("slot ready = %+v, %v", ready, err)
	}
	if err := publisher.SetReadDeadline(slotDeadline.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Read(make([]byte, 1)); err == nil {
		t.Fatal("Introduction slot remained open after its registration deadline")
	} else if networkError, ok := err.(net.Error); ok && networkError.Timeout() {
		t.Fatalf("Introduction slot did not close at its registration deadline: %v", err)
	}
}

func introductionTestAdmit(network, digest, nodeID [32]byte, deadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(authorization []byte, attachment, key [32]byte, role byte, receivedNode [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || receivedNode != nodeID || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected Introduction admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: [32]byte{12}, NetworkID: network, Digest: digest, Epoch: 6, TransitRole: route.IntroductionRole, TransitNodeID: nodeID, NotAfter: deadline}, nil
	}
}

func openIntroductionAttachment(t *testing.T, endpoint string, public, network, digest, nodeID [32]byte, epoch uint64, deadline time.Time, attachment [32]byte, authorization []byte) net.Conn {
	t.Helper()
	connection, err := route.OpenEndpointTransitAttachment(context.Background(), route.EndpointTransitAttachmentRequest{NetworkID: network, Digest: digest,
		AttachmentID: attachment, TransitNodeID: nodeID, TransitNodePublicKey: public, Epoch: epoch, TransitRole: route.IntroductionRole,
		Endpoint: endpoint, Deadline: deadline, Authorization: authorization})
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func submitIntroduction(t *testing.T, endpoint string, public, network, digest, nodeID [32]byte, deadline time.Time, attachment [32]byte, authorization []byte, sealed route.SealedIntroduction) route.IntroductionDeliveryResult {
	t.Helper()
	connection := openIntroductionAttachment(t, endpoint, public, network, digest, nodeID, 6, deadline, attachment, authorization)
	defer connection.Close()
	if err := route.WriteSealedIntroduction(connection, sealed); err != nil {
		t.Fatal(err)
	}
	result, err := route.ReadIntroductionDeliveryResult(connection)
	if err != nil || result.AttachmentID != attachment {
		t.Fatalf("delivery result = %+v, %v", result, err)
	}
	return result
}

func TestIntroductionPlanRejectsZeroCapacity(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 52, "introduction-invalid")
	_, err := StartIntroduction(IntroductionConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: certificate, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
		NodeID: [32]byte{3}, NodePublicKey: public, Epoch: 4, NotAfter: time.Now().UTC().Add(time.Minute).Truncate(time.Second), Admit: introductionTestAdmit([32]byte{1}, [32]byte{2}, [32]byte{3}, time.Now().UTC()),
		HandshakeLimit: 1, SlotLimit: 0, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err == nil {
		t.Fatal("Introduction accepted zero slot capacity")
	}
}

func TestIntroductionDutyUsesOnlyItsStateAssignment(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 53, "introduction-duty")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{21}, Epoch: 22, Digest: [32]byte{23}, Profile: route.Profile, NodeID: [32]byte{24},
		NodePublicKey: public, Assignment: "introduction", ProbeEndpoint: "127.0.0.1:30253", EpochValidFrom: now.Add(-time.Second),
		ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(30 * time.Second)}
	profile := IntroductionProfile{Certificate: certificate,
		HandshakeLimit: 2, SlotLimit: 3, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second}
	admit := introductionTestAdmit(snapshot.NetworkID, snapshot.Digest, snapshot.NodeID, snapshot.RecordValidUntil)
	plan, err := introductionDuty(profile, snapshot, admit)
	if err != nil || plan.ListenAddress != snapshot.ProbeEndpoint || !plan.NotAfter.Equal(snapshot.RecordValidUntil) || plan.SlotLimit != profile.SlotLimit {
		t.Fatalf("Introduction State duty = %+v, %v", plan, err)
	}
	snapshot.Assignment = "initiator"
	if _, err := introductionDuty(profile, snapshot, admit); err == nil {
		t.Fatal("Introduction accepted a different State assignment")
	}
}

func awaitIntroductionUsage(t *testing.T, running *Introduction, timeout time.Duration, predicate func(IntroductionUsage) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if usage := running.Usage(); predicate(usage) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Introduction usage did not satisfy predicate: %+v", running.Usage())
}
