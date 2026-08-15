package routeplan

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRejectsUnknownPlanFieldsAtTheInterface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`{"Role":"client","Unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a field outside the bounded role-local plan")
	}
}

func TestLoadNextAndCloseOwnsPublisherStream(t *testing.T) {
	root := t.TempDir()
	certificate, key := writeRoutePlanIdentity(t, root)
	socket := filepath.Join(os.TempDir(), "arp-"+time.Now().Format("150405.000000")+".sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	encoded := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	plan := actorPlan{Role: "publisher", ManifestDigest: encoded, NetworkID: encoded, EpochDigest: encoded,
		NodeID: encoded, Listen: "127.0.0.1:4605", Certificate: certificate, Key: key,
		UpstreamPin: encoded, Deadline: "1s", Stream: socket, RawAttachment: true}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plan.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	sequence, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	step, ok, err := sequence.Next()
	if err != nil || !ok || step.Actor.Stream == nil {
		t.Fatalf("construct publisher step: ok=%v err=%v", ok, err)
	}
	select {
	case peer := <-accepted:
		_ = peer.Close()
		t.Fatal("publisher registered its stream before upstream traffic")
	case <-time.After(20 * time.Millisecond):
	}
	writeDone := make(chan error, 1)
	go func() {
		_, writeErr := step.Actor.Stream.Write([]byte{7})
		writeDone <- writeErr
	}()
	peer := <-accepted
	defer peer.Close()
	buffer := make([]byte, 1)
	if _, err := peer.Read(buffer); err != nil || buffer[0] != 7 {
		t.Fatalf("read deferred publisher stream: value=%v err=%v", buffer, err)
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("write deferred publisher stream: %v", err)
	}
	halfCloser, ok := step.Actor.Stream.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("deferred publisher stream lost half-close support")
	}
	if err := halfCloser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if count, err := peer.Read(buffer); count != 0 || err == nil {
		t.Fatalf("publisher stream half-close was not forwarded: count=%d err=%v", count, err)
	}
	if err := step.Close(); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if count, err := peer.Read(make([]byte, 1)); count != 0 || err == nil {
		t.Fatalf("Step.Close did not close its owned stream: count=%d err=%v", count, err)
	}
	if _, ok, err := sequence.Next(); err != nil || ok {
		t.Fatalf("bounded sequence did not terminate: ok=%v err=%v", ok, err)
	}
}

func writeRoutePlanIdentity(t *testing.T, root string) (string, string) {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(9), Subject: pkix.Name{CommonName: "routeplan.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certificate, key := filepath.Join(root, "cert.pem"), filepath.Join(root, "key.pem")
	if err := os.WriteFile(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certificate, key
}

func TestAttachmentPlansApplyOnlyBoundedRoleLocalChanges(t *testing.T) {
	base := actorPlan{Role: "rendezvous", UpstreamPin: "initial-upstream", NextNodeID: "initial-next-id",
		Next: "initial-next", NextPin: "initial-next-pin", AttachmentPlans: []attachmentPlan{
			{UpstreamPin: "first-upstream", NextNodeID: "first-next-id", Next: "first-next", NextPin: "first-next-pin"},
			{UpstreamPin: "second-upstream", NextNodeID: "second-next-id", Next: "second-next", NextPin: "second-next-pin"},
		}}
	if err := base.validateRoleLocal(); err != nil {
		t.Fatal(err)
	}
	second, err := base.attachmentPlan(1)
	if err != nil {
		t.Fatal(err)
	}
	if second.UpstreamPin != "second-upstream" || second.NextNodeID != "second-next-id" ||
		second.Next != "second-next" || second.NextPin != "second-next-pin" || len(second.AttachmentPlans) != 0 {
		t.Fatalf("second role-local attachment differs: %+v", second)
	}
}

func TestAttachmentPlansRejectUnboundedOrCrossRoleChanges(t *testing.T) {
	tests := []actorPlan{
		{Role: "rendezvous", Attachments: 2, AttachmentPlans: []attachmentPlan{{}}},
		{Role: "rendezvous", AttachmentPlans: []attachmentPlan{{}, {}, {}, {}, {}}},
		{Role: "rendezvous", AttachmentPlans: []attachmentPlan{{ExcludedIdentities: []string{"client-only"}}}},
		{Role: "client", RawAttachment: true, Stream: "stream", AttachmentPlans: []attachmentPlan{{Next: "node-only"}}},
		{Role: "publisher", RawAttachment: true, Stream: "stream", AttachmentPlans: []attachmentPlan{{Seed: "client-only"}}},
		{Role: "client", ConcurrentAttachments: true, AttachmentPlans: []attachmentPlan{{}, {}}},
		{Role: "publisher", ConcurrentAttachments: true, AttachmentPlans: []attachmentPlan{{}}},
	}
	for _, input := range tests {
		if err := input.validateRoleLocal(); err == nil {
			t.Fatalf("invalid attachment sequence was accepted: %+v", input)
		}
	}
}

func TestConcurrentAttachmentPlanIsExplicitlyBoundedToPublisher(t *testing.T) {
	plan := actorPlan{Role: "publisher", ConcurrentAttachments: true,
		AttachmentPlans: []attachmentPlan{{Listen: "127.0.0.1:4605"}, {Listen: "127.0.0.1:4606"}}}
	if err := plan.validateRoleLocal(); err != nil {
		t.Fatal(err)
	}
	sequence := &Sequence{plan: plan}
	if !sequence.Concurrent() || sequence.plan.attachmentCount() != 2 {
		t.Fatal("bounded publisher concurrency was not retained")
	}
}

func TestOneListenerCapacityIsBoundedToSixteenAttachments(t *testing.T) {
	for _, role := range []string{"publisher", "initiator", "introduction", "rendezvous", "responder"} {
		if err := (actorPlan{Role: role, MaximumAttachments: 16, AttachmentTarget: 4,
			ResourceProfile: "h3-np1-v1"}).validateRoleLocal(); err != nil {
			t.Fatalf("%s capacity plan: %v", role, err)
		}
	}
	for _, plan := range []actorPlan{{Role: "client", MaximumAttachments: 2},
		{Role: "rendezvous", MaximumAttachments: 17},
		{Role: "responder", MaximumAttachments: 4, AttachmentTarget: 5},
		{Role: "initiator", MaximumAttachments: 4, ResourceProfile: "unknown"}} {
		if err := plan.validateRoleLocal(); err == nil {
			t.Fatalf("invalid listener capacity was accepted: %+v", plan)
		}
	}
}

func TestClientSequenceStopsWhenEndpointClosesAfterAnAttachment(t *testing.T) {
	sequence := &Sequence{plan: actorPlan{Role: "client", AttachmentPlans: []attachmentPlan{{}, {}}}, next: 1}
	closed := &routeStreamUnavailable{err: errors.New("scoped Route socket closed")}
	if !sequence.clientTerminal(closed) || (&Sequence{plan: sequence.plan}).clientTerminal(closed) ||
		(&Sequence{plan: actorPlan{Role: "publisher"}, next: 1}).clientTerminal(closed) ||
		sequence.clientTerminal(errors.New("state corruption")) {
		t.Fatal("client sequence terminal classification is not scoped to a post-Attachment stream close")
	}
}

func TestRolePlansRejectCrossRoleFieldsBeforeActorConstruction(t *testing.T) {
	tests := []actorPlan{
		{Role: "initiator", StateRoot: "forbidden"},
		{Role: "introduction", PublisherPin: "forbidden"},
		{Role: "rendezvous", ServiceKey: "forbidden"},
		{Role: "publisher", Seed: "forbidden"},
		{Role: "publisher", Next: "forbidden"},
		{Role: "client", Listen: "forbidden"},
		{Role: "client", ServiceCertificate: "forbidden"},
		{Role: "initiator", RawAttachment: true},
		{Role: "client", RawAttachment: true},
		{Role: "client", RawAttachment: true, Stream: "stream", PublisherPin: "forbidden"},
		{Role: "publisher", RawAttachment: true, Stream: "stream", ServiceCertificate: "forbidden"},
		{Role: "client", RawAttachment: true, Stream: "stream", Attachments: 5},
	}
	for _, input := range tests {
		if err := input.validateRoleLocal(); err == nil {
			t.Fatalf("cross-role plan was accepted: %+v", input)
		}
	}
}
