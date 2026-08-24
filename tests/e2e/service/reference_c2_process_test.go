package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestReferenceC2RunsUserAndPublisherInSeparateProcesses(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(20 * time.Second)
	network, digest := referenceC2ID(1), referenceC2ID(2)
	introductionID, rendezvousID := referenceC2ID(3), referenceC2ID(4)
	responderID, initiatorID := referenceC2ID(5), referenceC2ID(6)
	introductionCertificate, introductionPublic := referenceC2Certificate(t, 3, "introduction")
	rendezvousCertificate, rendezvousPublic := referenceC2Certificate(t, 4, "rendezvous")
	responderCertificate, responderPublic := referenceC2Certificate(t, 5, "responder")
	initiatorCertificate, initiatorPublic := referenceC2Certificate(t, 6, "initiator")
	introductionAddress, rendezvousAddress := referenceC2Address(t), referenceC2Address(t)
	responderAddress, initiatorAddress := referenceC2Address(t), referenceC2Address(t)
	join, reachability := referenceC2ID(7), referenceC2ID(8)
	slotAttachment, serviceAttachment := referenceC2ID(9), referenceC2ID(10)
	slotAuthorization, responderAuthorization, invite := "publisher-slot", "publisher-responder", "entry-invite"
	inviteID := referenceC2ID(11)

	rendezvous, err := node.StartRendezvous(node.RendezvousConfig{ListenAddress: rendezvousAddress, Certificate: rendezvousCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: rendezvousID, NodePublicKey: rendezvousPublic, Epoch: 10, NotAfter: deadline,
		Peers: []node.RendezvousPeer{{NodeID: initiatorID, PublicKey: initiatorPublic, Role: route.InitiatorRole},
			{NodeID: responderID, PublicKey: responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer rendezvous.Close()
	initiator, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: initiatorAddress, Certificate: initiatorCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: initiatorID, NodePublicKey: initiatorPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous: node.InitiatorPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
		Admit: func(raw []byte, attachment, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
			if string(raw) != invite || attachment != serviceAttachment || key == [32]byte{} || !notAfter.Equal(deadline) {
				return route.EntryAdmission{}, errors.New("unexpected process Entry admission")
			}
			return route.EntryAdmission{InviteID: inviteID, NetworkID: network, Digest: digest, Epoch: 10, InitiatorNodeID: initiatorID, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := node.StartResponder(node.ResponderConfig{ListenAddress: responderAddress, Certificate: responderCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: responderID, NodePublicKey: responderPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous: node.ResponderPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
		Admit: func(raw []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
			if string(raw) != responderAuthorization || attachment != serviceAttachment || key == [32]byte{} || role != route.ResponderRole || nodeID != responderID || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected process Responder admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: referenceC2ID(12), NetworkID: network, Digest: digest, Epoch: 10,
				TransitRole: route.ResponderRole, TransitNodeID: responderID, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	introduction, err := node.StartIntroduction(node.IntroductionConfig{ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 10, NotAfter: deadline,
		Admit: func(raw []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
			if len(raw) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || nodeID != introductionID || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected process Introduction admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: referenceC2ID(13), NetworkID: network, Digest: digest, Epoch: 10,
				TransitRole: route.IntroductionRole, TransitNodeID: introductionID, NotAfter: deadline}, nil
		}, HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	root := t.TempDir()
	publicationPath := filepath.Join(root, "publication.json")
	configPath := filepath.Join(root, "reference-c2.json")
	fixture := map[string]any{
		"Schema": "ardents-e2e-reference-c2-v1", "Network": referenceC2Hex(network), "Digest": referenceC2Hex(digest),
		"Epoch": 10, "Deadline": deadline.Format(time.RFC3339), "PublicationPath": publicationPath, "PublisherRoot": filepath.Join(root, "publisher-state"),
		"Introduction": referenceC2Peer(introductionID, introductionPublic, introductionAddress), "Rendezvous": referenceC2Peer(rendezvousID, rendezvousPublic, rendezvousAddress),
		"Responder": referenceC2Peer(responderID, responderPublic, responderAddress), "Initiator": referenceC2Peer(initiatorID, initiatorPublic, initiatorAddress),
		"JoinHandle": referenceC2Hex(join), "Reachability": referenceC2Hex(reachability), "SlotAttachment": referenceC2Hex(slotAttachment),
		"ServiceAttachment": referenceC2Hex(serviceAttachment), "SlotAuthorization": slotAuthorization, "ResponderAuthorization": responderAuthorization,
		"InviteID": referenceC2Hex(inviteID), "Invite": invite,
	}
	raw, err := json.Marshal(fixture)
	if err != nil || os.WriteFile(configPath, raw, 0o600) != nil {
		t.Fatal("write process C2 fixture configuration")
	}
	binary := buildE2EFixtureCommand(t, "reference-c2")
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	publisher := startCommand(ctx, root, binary, "publisher", configPath)
	referenceC2WaitForFile(t, ctx, publicationPath)
	user := startCommand(ctx, root, binary, "user", configPath)
	processes := map[string]commandResult{"user": <-user, "publisher": <-publisher}
	for role, process := range processes {
		if process.err != nil {
			t.Fatalf("C2 %s Endpoint process failed: %v\n%s", role, process.err, process.output)
		}
		var observed struct {
			Schema, Role, Class string
			Passed              bool
		}
		if err := json.Unmarshal(process.output, &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" || !observed.Passed || observed.Class == "" {
			t.Fatalf("C2 Endpoint process result = %q / %+v / %v", process.output, observed, err)
		}
	}
	drain, drainCancel := context.WithTimeout(context.Background(), time.Second)
	defer drainCancel()
	for _, running := range []interface{ Drain(context.Context) error }{introduction, responder, initiator, rendezvous} {
		if err := running.Drain(drain); err != nil {
			t.Fatal(err)
		}
	}
	if usage := initiator.Usage(); usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Initiator terminal usage = %+v", usage)
	}
	if usage := responder.Usage(); usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Responder terminal usage = %+v", usage)
	}
	if usage := rendezvous.Usage(); usage.CompletedPairs != 1 || usage.ActivePairs != 0 || usage.Connections != 0 {
		t.Fatalf("Rendezvous terminal usage = %+v", usage)
	}
}

func referenceC2WaitForFile(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Publisher did not publish before deadline: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func referenceC2Certificate(t *testing.T, serial int64, name string) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private, Leaf: leaf}, fixed
}

func referenceC2Address(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func referenceC2Peer(nodeID, public [32]byte, endpoint string) map[string]string {
	return map[string]string{"NodeID": referenceC2Hex(nodeID), "PublicKey": referenceC2Hex(public), "Endpoint": endpoint}
}

func referenceC2Hex(value [32]byte) string { return hex.EncodeToString(value[:]) }

func referenceC2ID(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
