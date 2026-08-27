//go:build h4_2_exact_candidate

package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	serviceconn "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// TestH42ExactCandidateProductNodesOwnTCPCarrier binds the exact Linux
// Endpoint candidate's local Route Attachments to State-run product Initiator
// and Responder processes. Those Node processes, not the test adapter, open
// their selected TCP/TLS legs to the product Rendezvous. The signed State
// contains one explicit Carrier Profile and therefore supplies no fallback.
func TestH42ExactCandidateProductNodesOwnTCPCarrier(t *testing.T) {
	candidate := exactCandidatePath(t)
	nodeBinary := exactCandidateNodePath(t)
	fixture := newRecoveryProcessFixture(t)
	publishBinary := buildE2EFixtureCommand(t, "publish-app")
	streamBinary := buildE2EFixtureCommand(t, "stream-app")
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()

	publisher := startServiceProcess(t, ctx, candidate, fixture.root, fixture.publisherPlan)
	runCommand(t, ctx, fixture.root, publishBinary, "publish", fixture.administration)
	client := startServiceProcess(t, ctx, candidate, fixture.root, fixture.clientPlan)
	product := startExactCandidateProductRoute(t, ctx, nodeBinary)

	clientRoute, publisherRoute, err := dialRoutePair(ctx, fixture.clientRoute, fixture.publisherRoute)
	if err != nil {
		t.Fatal(err)
	}
	initiator, responder, err := product.open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	routeResult := make(chan error, 1)
	go func() {
		results := make(chan error, 2)
		go func() { results <- bridgeExactCandidateLane(ctx, clientRoute, initiator) }()
		go func() { results <- bridgeExactCandidateLane(ctx, responder, publisherRoute) }()
		routeResult <- errors.Join(<-results, <-results)
	}()
	publisherApp := startCommand(ctx, fixture.root, streamBinary, "run", "publisher", fixture.publisherApplication,
		fixture.publisherSeed, fixture.clientSeed, "0", "8388608")
	clientApp := startCommand(ctx, fixture.root, streamBinary, "run", "client", fixture.clientApplication,
		fixture.clientSeed, fixture.publisherSeed, "8388608", "0")

	clientApplication := decodeApplicationResult(t, <-clientApp)
	publisherApplication := decodeApplicationResult(t, <-publisherApp)
	if err := <-routeResult; err != nil {
		t.Fatalf("exact candidate product TCP/TLS Route failed: %v", err)
	}
	var clientResult, publisherResult serviceconn.RuntimeResult
	client.finish(t, &clientResult)
	publisher.finish(t, &publisherResult)
	assertExactCandidateResults(t, fixture, clientResult, publisherResult, clientApplication, publisherApplication)
	referenceC2StopProductNodes(t, product.nodes)
	t.Logf("H4-2 exact product Route: ardents_sha256=%s ardents_node_sha256=%s state=%s carrier_profile=%s application_bytes=%d",
		exactCandidateDigest(t, candidate), exactCandidateDigest(t, nodeBinary), hex.EncodeToString(product.fixture.digest[:]), route.CarrierTCP, 8<<20)
}

type exactCandidateProductRoute struct {
	fixture                          referenceC2StateFixture
	nodes                            map[string]*referenceC2ProductNode
	initiator, rendezvous, responder referenceC2StateRecord
	initiatorCandidate               entry.Candidate
	presentation                     entry.Presentation
	responderGrant                   []byte
	responderCertificate             tls.Certificate
	attachment                       [32]byte
	deadline                         time.Time
}

func startExactCandidateProductRoute(t *testing.T, ctx context.Context, nodeBinary string) exactCandidateProductRoute {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(30 * time.Second)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	addresses := referenceC2Addresses(t, 5)
	records := map[string]referenceC2StateRecord{
		"introduction":           exactCandidateStateRecord(t, "introduction", 3, addresses[0]),
		"rendezvous":             exactCandidateStateRecord(t, "rendezvous", 4, addresses[1]),
		"responder":              exactCandidateStateRecord(t, "responder", 5, addresses[2]),
		"initiator":              exactCandidateStateRecord(t, "initiator", 6, addresses[3]),
		"destination-resolution": exactCandidateStateRecord(t, "destination-resolution", 13, addresses[4]),
	}
	stateFixture := newReferenceC2StateFixture(t, now, deadline, authority, records)
	root := t.TempDir()
	sources := referenceC2StartStateSources(t, ctx, nodeBinary, stateFixture, root)
	stateRoots := make(map[string]string, 3)
	var initiatorCandidate entry.Candidate
	for _, role := range []string{"rendezvous", "initiator", "responder"} {
		stateRoots[role] = filepath.Join(root, role+"-state")
		accepted := referenceC2AcceptState(t, stateFixture, stateRoots[role], role)
		if role == "initiator" {
			initiatorCandidate = accepted
		}
	}
	invite := referenceC2EntryInvite(t, records["initiator"].material, stateFixture.network, stateFixture.digest,
		stateFixture.epoch, initiatorCandidate, deadline, now)
	attachment := referenceC2ID(10)
	grant, certificate := exactCandidateTransitGrant(t, authority, stateFixture.network, stateFixture.digest, stateFixture.epoch,
		records["responder"].nodeID, route.ResponderRole, attachment, deadline)
	nodes := make(map[string]*referenceC2ProductNode, 3)
	for _, role := range []string{"rendezvous", "initiator", "responder"} {
		nodes[role] = referenceC2StartProductNodeWithLimits(t, ctx, nodeBinary, root, stateFixture, role, stateRoots[role],
			sources.endpoints, sources.client, 14*time.Second, 16<<20)
		if err := referenceC2WaitForProductNodeReady(ctx, nodes[role]); err != nil {
			t.Fatalf("exact-candidate product Node %s: %v\n%s", role, err, nodes[role].stderr.String())
		}
		if got := nodes[role].exactCandidateReadyEvent().CarrierProfile; got != string(route.CarrierTCP) {
			t.Fatalf("product Node %s selected Carrier %q", role, got)
		}
	}
	return exactCandidateProductRoute{fixture: stateFixture, nodes: nodes, initiator: records["initiator"],
		rendezvous: records["rendezvous"], responder: records["responder"], initiatorCandidate: initiatorCandidate,
		presentation: entry.Presentation{InviteID: referenceC2ID(11), Invite: invite}, responderGrant: grant,
		responderCertificate: certificate, attachment: attachment, deadline: deadline}
}

func exactCandidateStateRecord(t *testing.T, role string, marker byte, endpoint string) referenceC2StateRecord {
	t.Helper()
	record := referenceC2StateRecord{role: role, nodeID: referenceC2ID(marker), material: referenceC2Certificate(t, int64(marker), "exact-"+role),
		endpoint: endpoint, family: "h42-" + role}
	if role == "rendezvous" {
		record.carrier = string(route.CarrierTCP)
	}
	return record
}

func (product exactCandidateProductRoute) open(ctx context.Context) (net.Conn, net.Conn, error) {
	initiator, cleanup, err := route.OpenEntryAttachment(ctx,
		exactCandidateEntryAcquirer{candidate: product.initiatorCandidate, presentation: product.presentation},
		route.EntryAttachmentRequest{NetworkID: product.fixture.network, Digest: product.fixture.digest,
			AttachmentID: product.attachment, Epoch: product.fixture.epoch, Deadline: product.deadline})
	if err != nil {
		return nil, nil, err
	}
	setup := route.RelaySetup{NetworkID: product.fixture.network, Digest: product.fixture.digest, Epoch: product.fixture.epoch,
		AttachmentID: product.attachment, TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole,
		TransitNodeID: product.initiator.nodeID, NextNodeID: product.rendezvous.nodeID,
		NextNodePublicKey: product.rendezvous.material.public, NotAfter: product.deadline}
	if err := route.WriteRelaySetup(initiator, setup); err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	ready, err := route.ReadRelayReady(initiator)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		return nil, nil, errors.Join(err, setup.VerifyRelayReady(ready), cleanup())
	}
	responder, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{
		NetworkID: product.fixture.network, Digest: product.fixture.digest, Epoch: product.fixture.epoch,
		AttachmentID: product.attachment, TransitNodeID: product.responder.nodeID,
		TransitNodePublicKey: product.responder.material.public, TransitRole: route.ResponderRole,
		Endpoint: product.responder.endpoint, Deadline: product.deadline, Authorization: product.responderGrant,
		ClientCertificate: product.responderCertificate})
	if err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	return initiator, responder, nil
}

func (process *referenceC2ProductNode) exactCandidateReadyEvent() referenceC2ProductNodeEvent {
	process.eventMu.Lock()
	defer process.eventMu.Unlock()
	return process.ready
}

type exactCandidateEntryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input exactCandidateEntryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}

func exactCandidateTransitGrant(t *testing.T, authority ed25519.PrivateKey, network, digest [32]byte, epoch uint64,
	nodeID [32]byte, role byte, attachment [32]byte, deadline time.Time) ([]byte, tls.Certificate) {
	t.Helper()
	certificate, err := tlsCertificate(referenceC2Certificate(t, 92, "exact-transit-client"))
	if err != nil || len(certificate.Certificate) != 1 {
		t.Fatal("create exact-candidate transit certificate")
	}
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	keyDigest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(authority.Public().(ed25519.PublicKey)),
		GrantID: referenceC2ID(92), NetworkID: network, Digest: digest, AttachmentID: attachment, TransitNodeID: nodeID,
		ClientKeyDigest: keyDigest, Epoch: epoch, TransitRole: role, NotAfter: deadline}, authority)
	if err != nil {
		t.Fatal(err)
	}
	return grant, certificate
}

func bridgeExactCandidateLane(ctx context.Context, left, right io.ReadWriteCloser) error {
	stop := context.AfterFunc(ctx, func() { _ = left.Close(); _ = right.Close() })
	defer stop()
	results := make(chan error, 2)
	go func() { _, err := io.Copy(right, left); results <- err }()
	go func() { _, err := io.Copy(left, right); results <- err }()
	first := <-results
	_ = left.Close()
	_ = right.Close()
	second := <-results
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return benignExactCandidateBridgeError(first, second)
}

func benignExactCandidateBridgeError(values ...error) error {
	for _, err := range values {
		if class, classified := route.CarrierFailureClassOf(err); classified && class == route.CarrierFailureClosed {
			continue
		}
		if err := benignBridgeError(err); err != nil {
			return err
		}
	}
	return nil
}

func exactCandidatePath(t *testing.T) string {
	return exactCandidateBinaryPath(t, "ARDENTS_E2E_PRODUCT_ARDENTS", "ARDENTS_H4_2_CANDIDATE_SHA256")
}

func exactCandidateNodePath(t *testing.T) string {
	return exactCandidateBinaryPath(t, "ARDENTS_E2E_PRODUCT_ARDENTS_NODE", "ARDENTS_H4_2_NODE_SHA256")
}

func exactCandidateBinaryPath(t *testing.T, pathEnvironment, digestEnvironment string) string {
	t.Helper()
	path := os.Getenv(pathEnvironment)
	if info, err := os.Stat(path); path == "" || err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s must name an exact product binary", pathEnvironment)
	}
	expected := os.Getenv(digestEnvironment)
	if expected == "" || exactCandidateDigest(t, path) != expected {
		t.Fatalf("exact product binary digest differs from %s=%q", digestEnvironment, expected)
	}
	return path
}

func exactCandidateDigest(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func assertExactCandidateResults(t *testing.T, fixture recoveryProcessFixture, clientResult, publisherResult serviceconn.RuntimeResult,
	clientApplication, publisherApplication applicationObservation) {
	t.Helper()
	for role, result := range map[string]serviceconn.RuntimeResult{"client": clientResult, "publisher": publisherResult} {
		if result.Class != "clean service connection close" || result.AuthenticatedTarget != fixture.target ||
			result.RouteGeneration != 1 || result.RecoveryCount != 0 || result.RouteAttachmentsAccepted != 1 ||
			result.ApplicationIPCAccepts != 1 || result.AcceptedBytes != result.AcknowledgedBytes {
			t.Fatalf("exact candidate %s result is invalid: %+v", role, result)
		}
	}
	if clientApplication.Terminal != "success" || publisherApplication.Terminal != "success" ||
		clientApplication.SentDigest != publisherApplication.ReceivedDigest || publisherApplication.ReceivedBytes != 8<<20 ||
		clientApplication.ResultClass != "clean service connection close" || publisherApplication.ResultClass != "clean service connection close" {
		t.Fatalf("exact candidate Application bytes or terminal changed: client=%+v publisher=%+v", clientApplication, publisherApplication)
	}
}
