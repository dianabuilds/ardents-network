package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
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
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// This retained process tracer enters only through the headless runtime plan
// and local Administration socket. The two role-scoped Grants remain inside
// Endpoint while one real issuer budget observes their sequential acquisition.
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
	confidence := filepath.Join(directory, "time-confidence")
	if err := os.WriteFile(confidence, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	maintainPublisherProcessConfidence(t, confidence)
	entryRoot := filepath.Join(directory, "entry")
	importAlphaRuntimeEntry(t, entryRoot, rolesRoot, confidence, network)
	corpusPublic, corpusRoot := prepareAlphaRuntimeCorpus(t, directory, networkID)
	instanceRoot := publisherProcessInstance(t, filepath.Join(directory, "service-instance"), networkID, now, notAfter)
	transitRoot := filepath.Join(directory, "transit-acquisition")
	applicationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("pua-%d.sock", time.Now().UnixNano()))
	administrationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("pus-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(applicationSocket); _ = os.Remove(administrationSocket) })
	plan := map[string]any{"schema": "ardents-headless-runtime-v1", "network_state_root": network.root,
		"entry_state_root": entryRoot, "transit_acquisition_root": transitRoot,
		"application_socket": applicationSocket, "administration_socket": administrationSocket,
		"publication_root": filepath.Join(directory, "publication"), "service_instance_root": instanceRoot,
		"alpha_corpus_state_root": corpusRoot, "local_role_state_root": rolesRoot, "time_confidence_file": confidence,
		"network_id": hex32(networkID), "network_authorities": []string{hex.EncodeToString(network.authorityPublic)},
		"network_threshold": 1, "network_profile": route.Profile, "alpha_corpus_authority": hex.EncodeToString(corpusPublic),
		"alpha_cohort": "runtime-test", "broker_id": hex32([32]byte{71}), "connection_principal": hex32([32]byte{72}),
		"administration_principal": hex32([32]byte{74}), "bytes_each_direction": 4096}
	for _, forbidden := range []string{"grant", "private_key", "transit_role", "transit_node_id", "introduction_node_id", "rendezvous_node_id", "responder_node_id", "route_plan"} {
		if _, exposed := plan[forbidden]; exposed {
			t.Fatalf("headless caller plan exposes %q", forbidden)
		}
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(directory, "publisher-runtime.json")
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(confidence, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	writer := &alphaRuntimeWriter{ready: make(chan struct{}, 1)}
	done := make(chan error, 1)
	go func() { done <- runHeadlessRuntime(ctx, planPath, writer) }()
	select {
	case <-writer.ready:
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

func maintainPublisherProcessConfidence(t *testing.T, path string) {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				done <- nil
				return
			case observed := <-ticker.C:
				if err := os.Chtimes(path, observed, observed); err != nil {
					done <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		if err := <-done; err != nil {
			t.Errorf("maintain publisher process time confidence: %v", err)
		}
	})
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
