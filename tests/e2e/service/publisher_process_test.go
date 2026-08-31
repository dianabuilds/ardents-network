package service_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// This retained process tracer enters through RunParticipant and its local
// Administration socket. The two role-scoped Grants remain inside Endpoint
// while one real issuer budget observes their sequential acquisition.
func TestHeadlessPublisherAcquiresIntroductionAndResponderFromOneIssuerBudget(t *testing.T) {
	directory := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	notAfter := now.Add(time.Hour)
	networkID := sha256.Sum256([]byte("publisher-process-network"))
	authority := publisherProcessPrivate(t)
	seed := sha256.Sum256([]byte("publisher-process-assignment"))
	initiatorAddress := publisherProcessAddress(t)
	peers := map[string]publisherProcessPeer{
		"initiator":              {nodeID: [32]byte{1}, private: publisherProcessPrivate(t), endpoint: initiatorAddress},
		"introduction":           {nodeID: [32]byte{2}, private: publisherProcessPrivate(t), endpoint: "127.0.0.1:2"},
		"rendezvous":             {nodeID: [32]byte{3}, private: publisherProcessPrivate(t), endpoint: "127.0.0.1:3"},
		"responder":              {nodeID: [32]byte{4}, private: publisherProcessPrivate(t), endpoint: "127.0.0.1:4"},
		"destination-resolution": {nodeID: [32]byte{5}, private: publisherProcessPrivate(t), endpoint: "127.0.0.1:5"},
		"transit-issuance":       {nodeID: [32]byte{6}, private: publisherProcessPrivate(t), endpoint: "127.0.0.1:6"},
	}
	issuerRoot := filepath.Join(directory, "issuer")
	initiatorPublic := publisherProcessPublic(peers["initiator"].private)
	receipt, err := credential.InitializeIssuerRoot(credential.IssuerRootConfig{Root: issuerRoot,
		NetworkID: networkID, NodeID: peers["transit-issuance"].nodeID, IdentityKey: peers["transit-issuance"].private,
		InitiatorNodeID: peers["initiator"].nodeID, InitiatorPublicKey: initiatorPublic,
		AssignmentNotAfter: notAfter, Budget: 2, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	network := preparePublisherCommandNetwork(t, directory, now, notAfter, networkID, authority, seed, peers,
		[]byte("unused destination profile"), receipt.Profile)
	profile, err := credential.DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	duty := credential.StateDuty{NetworkID: networkID, Digest: network.snapshot.Digest,
		IssuerNodeID: peers["transit-issuance"].nodeID, IssuerPublicKey: publisherProcessPublic(peers["transit-issuance"].private),
		InitiatorNodeID: peers["initiator"].nodeID, InitiatorPublicKey: initiatorPublic,
		GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: receipt.ProfileDigest,
		Epoch: network.snapshot.Epoch, NotAfter: notAfter}
	issuer, err := credential.OpenIssuerFromRoot(credential.RootIssuerConfig{Root: issuerRoot, NetworkID: networkID,
		NodeID: duty.IssuerNodeID, IdentityKey: peers["transit-issuance"].private,
		CurrentDuty: func() (credential.StateDuty, bool) { return duty, true }, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer issuer.Close()
	var requests, active, maximumActive atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		current := active.Add(1)
		for previous := maximumActive.Load(); current > previous && !maximumActive.CompareAndSwap(previous, current); previous = maximumActive.Load() {
		}
		defer active.Add(-1)
		issuer.Handler().ServeHTTP(writer, request)
	})
	issuerServer := httptest.NewUnstartedServer(handler)
	issuerServer.TLS, err = issuer.TLSConfig(publisherProcessCertificate(t, peers["transit-issuance"].private, 6))
	if err != nil {
		t.Fatal(err)
	}
	issuerServer.StartTLS()
	defer issuerServer.Close()
	initiator, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: initiatorAddress,
		Certificate: publisherProcessCertificate(t, peers["initiator"].private, 1), NetworkID: networkID,
		EpochDigest: network.snapshot.Digest, NodeID: peers["initiator"].nodeID, NodePublicKey: initiatorPublic,
		Epoch: network.snapshot.Epoch, NotAfter: notAfter,
		Rendezvous: node.InitiatorPeer{NodeID: peers["rendezvous"].nodeID,
			PublicKey: publisherProcessPublic(peers["rendezvous"].private), Endpoint: peers["rendezvous"].endpoint,
			CarrierProfile: route.CarrierTCP},
		CredentialIssuer: node.CredentialIssuer{NodeID: duty.IssuerNodeID, PublicKey: duty.IssuerPublicKey,
			ProfileDigest: duty.ProfileDigest, URL: issuerServer.URL},
		Admit: func(invite []byte, attachment, key [32]byte, deadline time.Time) (route.EntryAdmission, error) {
			if len(invite) == 0 || attachment == [32]byte{} || key == [32]byte{} || deadline.After(notAfter) {
				return route.EntryAdmission{}, errors.New("publisher process Entry admission is invalid")
			}
			return route.EntryAdmission{InviteID: sha256.Sum256(invite), NetworkID: networkID,
				Digest: network.snapshot.Digest, InitiatorNodeID: peers["initiator"].nodeID,
				Epoch: network.snapshot.Epoch, NotAfter: deadline}, nil
		}, HandshakeLimit: 4, RelayLimit: 2, RelayByteLimit: 16 << 10,
		AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()

	rolesRoot := filepath.Join(directory, "local-roles")
	roles, err := localroles.Open(localroles.Config{Root: rolesRoot, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	entryRoot := filepath.Join(directory, "entry")
	importPublisherProcessEntry(t, entryRoot, rolesRoot, network)
	corpusPublic, corpusRoot := preparePublisherProcessCorpus(t, directory, networkID)
	instanceRoot := publisherProcessInstance(t, filepath.Join(directory, "service-instance"), networkID, now, notAfter)
	transitRoot := filepath.Join(directory, "transit-acquisition")
	applicationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("pua-%d.sock", time.Now().UnixNano()))
	administrationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("pus-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(applicationSocket); _ = os.Remove(administrationSocket) })
	ctx, cancel := context.WithCancel(t.Context())
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- endpoint.RunParticipant(ctx, endpoint.ParticipantRuntimeConfig{
			Network: state.Config{Root: network.root, NetworkID: networkID,
				Authorities: map[[32]byte]ed25519.PublicKey{network.snapshot.EpochAuthorityIDs[0]: network.authorityPublic},
				Threshold:   1, AcceptedProfile: route.Profile},
			EntryRoot: entryRoot, TransitAcquisitionRoot: transitRoot,
			AlphaCorpusRoot: corpusRoot, AlphaCorpusAuthority: corpusPublic, AlphaCohort: "runtime-test",
			LocalRoleRoot: rolesRoot, ApplicationAddress: applicationSocket, AdministrationAddress: administrationSocket,
			PublicationRoot: filepath.Join(directory, "publication"), ServiceInstanceRoot: instanceRoot,
			BrokerID: [32]byte{71}, ConnectionPrincipal: [32]byte{72}, AdministrationPrincipal: [32]byte{74},
			BytesEachDirection: 4096, Clock: time.Now, TimeConfident: func() bool { return true },
			Observe: func(event endpoint.ParticipantRuntimeEvent) error {
				if event.Kind == "ready" {
					ready <- struct{}{}
				}
				return nil
			},
		})
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("publisher runtime stopped before ready: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("publisher runtime did not become ready")
	}
	requestContext, requestCancel := context.WithTimeout(t.Context(), 12*time.Second)
	_, publishErr := administration.Request(requestContext, administrationSocket, administration.Publish)
	requestCancel()
	if publishErr == nil {
		t.Fatal("Publisher unexpectedly opened its deliberately absent Introduction listener")
	}
	if requests.Load() != 2 || maximumActive.Load() != 1 {
		t.Fatalf("common issuer observations = requests %d maximum concurrent %d, Introduction %q, Responder %q, Initiator %+v",
			requests.Load(), maximumActive.Load(), publisherProcessPhase(transitRoot),
			publisherProcessPhase(filepath.Join(transitRoot, "responder")), initiator.Usage())
	}
	if _, err := os.Stat(filepath.Join(transitRoot, "responder")); err != nil {
		t.Fatalf("separate Responder acquisition journal was not retained: %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func publisherProcessPhase(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		return err.Error()
	}
	var value struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return err.Error()
	}
	return value.Phase
}

func importPublisherProcessEntry(t *testing.T, root, rolesRoot string, network publisherProcessNetwork) {
	t.Helper()
	opened, err := state.Open(state.Config{Root: network.root, NetworkID: network.snapshot.NetworkID,
		Authorities: map[[32]byte]ed25519.PublicKey{network.snapshot.EpochAuthorityIDs[0]: network.authorityPublic},
		Threshold:   1, AcceptedProfile: route.Profile, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	owner, err := entry.Open(entry.Config{Root: root, Current: func() (entry.View, error) {
		current, currentErr := opened.Current()
		if currentErr != nil {
			return entry.View{}, currentErr
		}
		return publisherProcessEntryView(current), nil
	}, Conflict: func(identity, family [32]byte) (bool, error) {
		return localroles.ReadConflict(rolesRoot, time.Now, identity, family)
	}, Clock: time.Now, TimeConfident: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	result, err := owner.Import(publisherProcessInvite(network, time.Now().UTC()))
	if err != nil || result.Class != entry.Accepted {
		t.Fatalf("import publisher process Entry = %+v, %v", result, err)
	}
}

func publisherProcessEntryView(current state.Snapshot) entry.View {
	view := entry.View{NetworkID: current.NetworkID, Epoch: current.Epoch, Digest: current.Digest,
		Profile: current.Profile, Fresh: current.Freshness == "fresh"}
	for _, candidate := range current.Candidates[:current.CandidateCount] {
		view.Candidates = append(view.Candidates, entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			KeyID: candidate.KeyID, FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest,
			DomainProofDigest: candidate.DomainProofDigest, Endpoint: candidate.Endpoint, Capacity: candidate.Capacity,
			Domain: candidate.Domain, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
			AssignmentNotAfter: candidate.AssignmentNotAfter})
	}
	return view
}

func publisherProcessInvite(network publisherProcessNetwork, now time.Time) []byte {
	snapshot := network.snapshot
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(1))
	body.Write(snapshot.NetworkID[:])
	_ = binary.Write(&body, binary.BigEndian, snapshot.Epoch)
	body.Write(snapshot.Digest[:])
	writePublisherProcessBytes(&body, []byte(route.Profile), 1)
	candidate, _ := snapshot.BridgeCandidateByKey(snapshot.Candidates[0].KeyID)
	body.Write(candidate.KeyID[:])
	body.Write(candidate.NodeID[:])
	body.Write(candidate.FamilyID[:])
	body.Write(candidate.RecordDigest[:])
	body.Write(candidate.DomainProofDigest[:])
	_ = binary.Write(&body, binary.BigEndian, candidate.AssignmentNotAfter.Unix())
	_ = binary.Write(&body, binary.BigEndian, now.Add(-time.Minute).Unix())
	_ = binary.Write(&body, binary.BigEndian, now.Add(30*time.Minute).Unix())
	body.Write([]byte{1, 0, 0})
	var raw bytes.Buffer
	raw.WriteString("ardents-entry-invite-v1")
	_ = binary.Write(&raw, binary.BigEndian, uint16(body.Len()))
	raw.Write(body.Bytes())
	signed := append([]byte("ardents-entry-invite-signature-v1\x00"), body.Bytes()...)
	raw.Write(ed25519.Sign(network.nodePrivate, signed))
	return raw.Bytes()
}

func writePublisherProcessBytes(target *bytes.Buffer, raw []byte, width int) {
	if width == 1 {
		target.WriteByte(byte(len(raw)))
	} else {
		_ = binary.Write(target, binary.BigEndian, uint16(len(raw)))
	}
	target.Write(raw)
}

func preparePublisherProcessCorpus(t *testing.T, directory string, network [32]byte) (ed25519.PublicKey, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://runtime-test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "runtime-test", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{73}}}}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, raw)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(directory, "alpha-corpus")
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: root, Authority: public,
		Cohort: "runtime-test", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	if err := floor.Close(); err != nil {
		t.Fatal(err)
	}
	return public, root
}

func publisherProcessPrivate(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return private
}

func publisherProcessPublic(private ed25519.PrivateKey) [32]byte {
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	return public
}

func publisherProcessAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func publisherProcessCertificate(t *testing.T, private ed25519.PrivateKey, serial int64) tls.Certificate {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Unix(1, 0).UTC(),
		NotAfter: time.Unix(2_100_000_000, 0).UTC(), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}

func publisherProcessInstance(t *testing.T, rootPath string, network [32]byte, notBefore, notAfter time.Time) string {
	t.Helper()
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := instance.Initialize(instance.InitializeConfig{Root: rootPath, NetworkID: network,
		NotBefore: notBefore, NotAfter: notAfter})
	if err != nil {
		t.Fatal(err)
	}
	request, err := root.Request()
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	authority := publisherProcessPrivate(t)
	issued, err := (publication.Credential{InstancePublic: view.InstancePublic,
		IntroductionHPKEPublic: view.IntroductionPublic, Generation: 1, NotBefore: view.NotBefore, NotAfter: view.NotAfter,
		NetworkID: view.NetworkID, Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	response, err := instance.BuildResponse(request, issued)
	if err != nil {
		t.Fatal(err)
	}
	if acceptance, err := root.Accept(response); err != nil || acceptance.State != instance.StateAccepted {
		t.Fatalf("accept publisher process Service Instance = %+v, %v", acceptance, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	return rootPath
}
