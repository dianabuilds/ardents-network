//go:build h4_3b_multihost || h4_8_a11

package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestH43MultiHostDynamicPublisherApplication(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true})
}

func TestH43MultiHostPublisherWithdrawal(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherWithdrawal})
}

func TestH43MultiHostPublisherApplicationCrash(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherApplicationReset})
}

func TestH43MultiHostPublisherEndpointLoss(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherEndpointStop})
}

type h43RemoteStage struct {
	root, localRoot, localConfig, fixture   string
	publication, gatewayProfile, relayReady string
	proof                                   string
	alphaAuthority                          ed25519.PublicKey
	alphaPrivate                            ed25519.PrivateKey
	network                                 [32]byte
	alphaLink                               string
}

func runReferenceC2MultiHost(t *testing.T, scenario referenceC2Scenario) {
	t.Helper()
	if !scenario.transparentApplication || scenario.publisherOffline || scenario.rejectPublisherApplication || scenario.browserEntryDynamic {
		t.Fatal("H4-3B multi-host runner supports one transparent dynamic scenario")
	}
	environment := requireH43MultiHostEnvironment(t)
	status := openH48A11Status(t, scenario, environment)
	deadline := time.Now().UTC().Truncate(time.Second).Add(scenario.dynamicWorkload.timeBudget(90 * time.Second))
	if workload := scenario.dynamicWorkload; workload.configured() {
		t.Logf("A11 workload: cycles=%d interval_ms=%d cycle_deadline_ms=%d no_fallback_every=%d bytes_each_direction=%d",
			workload.Cycles, workload.IntervalMilliseconds, workload.CycleDeadlineMilliseconds, workload.NoFallbackEvery, workload.BytesEachDirection)
	}
	remote := h43RemoteC2{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	t.Cleanup(func() { status.retainRemoteFailure(t, remote) })
	stage := stageH43RemoteC2(t, environment, deadline, scenario)
	t.Logf("H4-3B multi-host inputs: revision=%s scenario=%s ports=%d-%d stage_sha256=%s fixture_sha256=%s node_sha256=%s config_sha256=%s runner_sha256=%s",
		h43Revision(t), h43ScenarioName(scenario.publisherTerminal), environment.port, environment.port+7, h43StageDigest(t, stage.root), h43Digest(t, filepath.Join(stage.root, "reference-c2")),
		h43Digest(t, filepath.Join(stage.root, "ardents-node")), h43Digest(t, filepath.Join(stage.root, "reference-c2.json")), h43Digest(t, filepath.Join(stage.root, "run.sh")))
	t.Logf("H4-3B multi-host local host envelope: goos=%s goarch=%s go=%s logical_cpus=%d", runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU())
	t.Logf("H4-3B multi-host remote host envelope: %s", remote.hostEnvelope(t))
	remote.start(t, stage.root)
	status.remoteReady(t, environment)
	remote.copyFile(t, remote.environment.remoteDirectory+"/publication.json", stage.publication, deadline)
	remote.copyFile(t, remote.environment.remoteDirectory+"/gateway-profile.json", stage.gatewayProfile, deadline)
	remote.copyFile(t, remote.environment.remoteDirectory+"/alpha-relay-ready.json", stage.relayReady, deadline)
	stageReferenceC2AlphaCorpus(t, h43LocalProductCommand(t, "ardents"), h43LocalProductCommand(t, "ardents-control"),
		stage.publication, stage.alphaAuthority, stage.alphaPrivate, filepath.Join(stage.localRoot, "alpha-floor"), stage.network, stage.alphaLink)

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	proofResult := make(chan error, 1)
	go func() {
		proofResult <- h43MirrorRemoteProof(ctx, remote, remote.environment.remoteDirectory+"/reference-resources", stage.proof)
	}()
	user := startKillableCommand(ctx, stage.localRoot, stage.fixture, "user", stage.localConfig)
	status.userReady(t, user)
	result := <-user.result
	status.retainUser(t, result)
	if result.err != nil {
		// A failed User cannot produce the remote Publisher proof. Cancel its
		// concurrent mirror before reporting the actual User failure, rather
		// than retaining a test worker until the complete soak deadline.
		h43AbortProofAfterUserFailure(cancel, proofResult)
		h43AssertUserResult(t, result, scenario)
		return
	}
	if err := <-proofResult; err != nil {
		t.Fatalf("mirror remote H4-3B resource proof: %v", err)
	}
	remote.complete(t)
	remote.wait(t)
	status.retainRemote(t, remote)
	h43AssertUserResult(t, result, scenario)
	assertReferenceC2DynamicWorkloadResult(t, scenario, result)
	h43AssertRemoteResults(t, remote, scenario)
	if scenario.productRendezvousRelay {
		h48A11AssertProductTransitEvidence(t, remote, scenario, result)
	}
	status.complete(t)
}

func h43AbortProofAfterUserFailure(cancel context.CancelFunc, proofResult <-chan error) {
	cancel()
	<-proofResult
}

func stageH43RemoteC2(t *testing.T, environment h43MultiHostEnvironment, deadline time.Time, scenario referenceC2Scenario) h43RemoteStage {
	t.Helper()
	root, localRoot := t.TempDir(), t.TempDir()
	h43BuildLinuxCommand(t, "ardents-node", filepath.Join(root, "ardents-node"))
	h43BuildLinuxFixture(t, filepath.Join(root, "reference-c2"))
	now := time.Now().UTC().Truncate(time.Second)
	introductionID, rendezvousID := referenceC2ID(3), referenceC2ID(4)
	responderID, initiatorID, gatewayID := referenceC2ID(5), referenceC2ID(6), referenceC2ID(13)
	introductionMaterial := referenceC2Certificate(t, 3, "introduction")
	rendezvousMaterial := referenceC2Certificate(t, 4, "rendezvous")
	responderMaterial := referenceC2Certificate(t, 5, "responder")
	initiatorMaterial := referenceC2Certificate(t, 6, "initiator")
	gatewayMaterial := referenceC2Certificate(t, 7, "gateway")
	introductionAddress := net.JoinHostPort(environment.host, fmt.Sprint(environment.port))
	rendezvousAddress := net.JoinHostPort(environment.host, fmt.Sprint(environment.port+1))
	initiatorAddress := net.JoinHostPort(environment.host, fmt.Sprint(environment.port+2))
	gatewayAddress := net.JoinHostPort(environment.host, fmt.Sprint(environment.port+3))
	responderAddress := net.JoinHostPort("127.0.0.1", fmt.Sprint(environment.port+5))
	transitPublic, transitPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stateFixture := newReferenceC2StateFixture(t, now, deadline, transitPrivate, map[string]referenceC2StateRecord{
		"introduction":           {role: "introduction", nodeID: introductionID, material: introductionMaterial, endpoint: introductionAddress, family: "reference-introduction"},
		"rendezvous":             {role: "rendezvous", nodeID: rendezvousID, material: rendezvousMaterial, endpoint: rendezvousAddress, family: "reference-rendezvous"},
		"responder":              {role: "responder", nodeID: responderID, material: responderMaterial, endpoint: responderAddress, family: "reference-responder"},
		"initiator":              {role: "initiator", nodeID: initiatorID, material: initiatorMaterial, endpoint: initiatorAddress, family: "reference-initiator"},
		"destination-resolution": {role: "destination-resolution", nodeID: gatewayID, material: gatewayMaterial, endpoint: gatewayAddress, family: "reference-gateway"},
	})
	clientAuthority := referenceC2SourceAuthority(t, "h43-source-client-root", 41)
	client := referenceC2SourceLeaf(t, clientAuthority, "h43-source-client", 42, false)
	clientCertificate, err := os.ReadFile(client.certificate)
	if err != nil {
		t.Fatal(err)
	}
	clientPrivate, err := os.ReadFile(client.privateKey)
	if err != nil {
		t.Fatal(err)
	}
	sources := make([]map[string]string, 2)
	for index := range sources {
		suffix := string(rune('a' + index))
		authority := referenceC2SourceAuthority(t, "h43-source-"+suffix+"-root", int64(43+index))
		server := referenceC2SourceLeaf(t, authority, "h43-source-"+suffix, int64(45+index), true)
		stateRoot := filepath.Join(root, "source-"+suffix+"-state")
		referenceC2AcceptState(t, stateFixture, stateRoot, "rendezvous")
		h43CopyFile(t, filepath.Join(root, "source-"+suffix+"-root.pem"), authority.rootPath)
		h43CopyFile(t, filepath.Join(root, "source-"+suffix+"-cert.pem"), server.certificate)
		h43CopyFile(t, filepath.Join(root, "source-"+suffix+"-key.pem"), server.privateKey)
		plan := map[string]any{"schema": "ardents-source-server-v1", "state_root": "/work/source-" + suffix + "-state",
			"local_role_state_root": "/work/source-" + suffix + "-roles", "network_id": referenceC2Hex(stateFixture.network),
			"authority_public": []string{hex.EncodeToString(transitPublic)}, "threshold": 1, "at": now.Format(time.RFC3339),
			"listen": net.JoinHostPort("127.0.0.1", fmt.Sprint(environment.port+6+index)), "server_certificate": "/work/source-" + suffix + "-cert.pem",
			"server_key": "/work/source-" + suffix + "-key.pem", "client_root": "/work/source-client-root.pem",
			"client_key_digests": []string{hex.EncodeToString(client.leafDigest[:])}, "materialization_index": 0, "native_rendezvous_profile": true}
		h43WriteJSON(t, filepath.Join(root, "source-"+suffix+"-plan.json"), plan)
		sources[index] = map[string]string{"Address": net.JoinHostPort("127.0.0.1", fmt.Sprint(environment.port+6+index)), "ServerName": "h43-source-" + suffix,
			"Root": authority.root, "LeafKeyDigest": hex.EncodeToString(server.leafDigest[:])}
	}
	h43CopyFile(t, filepath.Join(root, "source-client-root.pem"), clientAuthority.rootPath)
	join, reachability := referenceC2ID(7), referenceC2ID(8)
	slotAttachment, serviceAttachment, resolutionAttachment := referenceC2ID(9), referenceC2ID(10), referenceC2ID(12)
	slotCredential := referenceC2TransitCredential(t, transitPrivate, stateFixture.network, stateFixture.digest, stateFixture.epoch, introductionID, route.IntroductionRole, slotAttachment, deadline, 31)
	responderCredential := referenceC2TransitCredential(t, transitPrivate, stateFixture.network, stateFixture.digest, stateFixture.epoch, responderID, route.ResponderRole, serviceAttachment, deadline, 32)
	introductionCredential := referenceC2TransitCredential(t, transitPrivate, stateFixture.network, stateFixture.digest, stateFixture.epoch, introductionID, route.IntroductionRole, serviceAttachment, deadline, 33)
	stateRoots, stateMaterials := map[string]string{}, map[string]uint32{}
	var invite []byte
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		stateRoot := filepath.Join(root, role+"-state")
		candidate := referenceC2AcceptState(t, stateFixture, stateRoot, role)
		stateRoots[role], stateMaterials[role] = "/work/"+role+"-state", stateFixture.roles[role].materializationIndex
		if role == "initiator" {
			invite = referenceC2EntryInvite(t, initiatorMaterial, stateFixture.network, stateFixture.digest, stateFixture.epoch, candidate, deadline, now)
		}
	}
	if len(invite) == 0 {
		t.Fatal("H4-3B multi-host State fixture did not issue an Entry Invite")
	}
	alphaPublic, alphaPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture := map[string]any{"Schema": "ardents-e2e-reference-c2-v1", "Network": referenceC2Hex(stateFixture.network), "Digest": referenceC2Hex(stateFixture.digest),
		"Epoch": stateFixture.epoch, "Deadline": deadline.Format(time.RFC3339), "PublicationPath": "/work/publication.json", "PublisherRoot": "/work/publisher-state",
		"GatewayRoot": "/work/gateway-state", "GatewayProfilePath": "/work/gateway-profile.json", "ReadyRoot": "/work/ready", "CompletePath": "/work/complete",
		"ResourceProofPath": "/work/reference-resources", "PublisherApplicationAddress": "127.0.0.1:0", "PublisherApplicationAddressPath": "/work/publisher-application-address",
		"PublisherApplicationToken": referenceC2Hex(referenceC2ID(14)), "PublisherApplicationReady": "/work/publisher-application-ready",
		"Introduction": referenceC2Peer(introductionID, introductionMaterial, introductionAddress), "Rendezvous": referenceC2Peer(rendezvousID, rendezvousMaterial, rendezvousAddress),
		"Responder": referenceC2Peer(responderID, responderMaterial, responderAddress), "Initiator": referenceC2Peer(initiatorID, initiatorMaterial, initiatorAddress),
		"Gateway": referenceC2Peer(gatewayID, gatewayMaterial, gatewayAddress), "JoinHandle": referenceC2Hex(join), "Reachability": referenceC2Hex(reachability),
		"SlotAttachment": referenceC2Hex(slotAttachment), "ServiceAttachment": referenceC2Hex(serviceAttachment), "ResolutionAttachment": referenceC2Hex(resolutionAttachment),
		"TransitAuthority": hex.EncodeToString(transitPublic), "SlotCredential": slotCredential, "ResponderCredential": responderCredential, "IntroductionCredential": introductionCredential,
		"InviteID": referenceC2Hex(referenceC2ID(11)), "Invite": base64.RawStdEncoding.EncodeToString(invite), "TransitStateRoots": stateRoots, "TransitStateMaterials": stateMaterials,
		"TransitStateSources": sources, "TransitStateClient": map[string]string{"Certificate": string(clientCertificate), "PrivateKey": string(clientPrivate)},
		"AlphaCorpusAuthority": hex.EncodeToString(alphaPublic), "AlphaCorpusPrivate": base64.RawStdEncoding.EncodeToString(alphaPrivate), "AlphaCorpusFloorRoot": "/work/alpha-floor",
		"AlphaObserverCorpusFloorRoot": "/work/alpha-observer-floor", "AlphaServiceLink": "ardents-alpha://reference", "AlphaGatewayReadyPath": "/work/alpha-gateway-ready.json",
		"AlphaRelayReadyPath": "/work/alpha-relay-ready.json", "AlphaRelayListenAddress": net.JoinHostPort(environment.host, fmt.Sprint(environment.port+4)),
		"PublisherOffline": false, "TransparentApplication": true, "PublisherTerminal": scenario.publisherTerminal, "PublisherCrashReadyPath": "/work/publisher-crash-ready"}
	stageH48A11ProductTransit(t, root, fixture, stateFixture, rendezvousMaterial, client, sources, scenario)
	scenario.dynamicWorkload.addTo(fixture)
	referenceC2ConfigureHeldRoute(fixture, "/work", scenario.heldRoute)
	h43WriteJSON(t, filepath.Join(root, "reference-c2.json"), fixture)
	h43WriteFile(t, filepath.Join(root, "expected-terminal"), []byte(scenario.publisherTerminal+"\n"), 0o600)
	h43WriteFile(t, filepath.Join(root, "run.sh"), []byte(h43RemoteRunner()), 0o700)

	local := h43CloneFixture(t, fixture)
	referenceC2ConfigureHeldRoute(local, localRoot, scenario.heldRoute)
	local["PublicationPath"] = filepath.Join(localRoot, "publication.json")
	local["GatewayProfilePath"] = filepath.Join(localRoot, "gateway-profile.json")
	local["AlphaRelayReadyPath"] = filepath.Join(localRoot, "alpha-relay-ready.json")
	local["AlphaGatewayReadyPath"] = filepath.Join(localRoot, "alpha-gateway-ready.json")
	local["ResourceProofPath"] = filepath.Join(localRoot, "reference-resources")
	local["CompletePath"] = filepath.Join(localRoot, "complete")
	local["PublisherRoot"] = filepath.Join(localRoot, "publisher-state")
	local["GatewayRoot"] = filepath.Join(localRoot, "gateway-state")
	local["ReadyRoot"] = filepath.Join(localRoot, "ready")
	local["PublisherApplicationAddressPath"] = filepath.Join(localRoot, "publisher-application-address")
	local["PublisherApplicationReady"] = filepath.Join(localRoot, "publisher-application-ready")
	local["PublisherCrashReadyPath"] = filepath.Join(localRoot, "publisher-crash-ready")
	if scenario.productRendezvousRelay {
		local["CarrierRelayReadyPath"] = filepath.Join(localRoot, "carrier-relay-ready.json")
		local["CarrierRelayResetPath"] = filepath.Join(localRoot, "carrier-relay-reset")
		local["CarrierRelayResetResultPath"] = filepath.Join(localRoot, "carrier-relay-reset.json")
		if scenario.publisherTerminal == referenceC2PublisherApplicationReset {
			local["PublisherApplicationFaultReadyPath"] = filepath.Join(localRoot, "publisher-application-fault-ready")
			local["PublisherApplicationFaultReleasePath"] = filepath.Join(localRoot, "publisher-application-fault-release")
		}
		if scenario.transitFault != "" {
			local["TransitFaultReadyPath"] = filepath.Join(localRoot, "transit-fault-ready")
		}
	}
	local["AlphaCorpusFloorRoot"] = filepath.Join(localRoot, "alpha-floor")
	local["AlphaObserverCorpusFloorRoot"] = filepath.Join(localRoot, "alpha-observer-floor")
	localRoots := map[string]string{}
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		localRoots[role] = filepath.Join(localRoot, role+"-state")
	}
	local["TransitStateRoots"] = localRoots
	localConfig := filepath.Join(localRoot, "reference-c2-user.json")
	h43WriteJSON(t, localConfig, local)
	return h43RemoteStage{root: root, localRoot: localRoot, localConfig: localConfig, fixture: buildE2EFixtureCommand(t, "reference-c2"),
		publication: filepath.Join(localRoot, "publication.json"), gatewayProfile: filepath.Join(localRoot, "gateway-profile.json"), relayReady: filepath.Join(localRoot, "alpha-relay-ready.json"),
		proof: filepath.Join(localRoot, "reference-resources"), alphaAuthority: alphaPublic, alphaPrivate: alphaPrivate, network: stateFixture.network, alphaLink: "ardents-alpha://reference"}
}

func h43MirrorRemoteProof(ctx context.Context, remote h43RemoteC2, source, destination string) error {
	return h43TransferRemoteProof(ctx, source, destination, remote.readFileWhenAvailableContext, os.WriteFile)
}

func h43TransferRemoteProof(ctx context.Context, source, destination string, read func(context.Context, string) ([]byte, error), write func(string, []byte, os.FileMode) error) error {
	contents, err := read(ctx, source)
	if err != nil {
		return fmt.Errorf("read remote proof %q: %w", source, err)
	}
	if err := write(destination, contents, 0o600); err != nil {
		return fmt.Errorf("write local proof %q: %w", destination, err)
	}
	return nil
}

func h43AssertUserResult(t *testing.T, process commandResult, scenario referenceC2Scenario) {
	t.Helper()
	if process.err != nil {
		t.Fatalf("local H4-3B User failed: %v\n%s", process.err, process.output)
	}
	var observed struct {
		Schema, Role, Class string
		Passed              bool
	}
	line := strings.TrimSpace(string(process.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" || observed.Role != "user" || !observed.Passed || observed.Class == "" {
		t.Fatalf("local H4-3B User result = %q / %+v / %v", process.output, observed, err)
	}
	terminal := scenario.publisherTerminal
	if (terminal == referenceC2PublisherApplicationReset || terminal == referenceC2PublisherEndpointStop || scenario.transitFault != "") && observed.Class != "abrupt connection loss" {
		t.Fatalf("local H4-3B User terminal class = %q, want abrupt connection loss", observed.Class)
	}
	if terminal == referenceC2PublisherWithdrawal && observed.Class != "clean service connection close" {
		t.Fatalf("local H4-3B User withdrawal class = %q, want clean service connection close", observed.Class)
	}
}

func h43AssertRemoteResults(t *testing.T, remote h43RemoteC2, scenario referenceC2Scenario) {
	t.Helper()
	h48A11AssertRemoteRoleExitStatuses(t, remote, scenario)
	terminal := scenario.publisherTerminal
	roles := []string{"rendezvous", "initiator", "introduction", "responder", "gateway", "alpha-gateway", "alpha-relay"}
	if scenario.productRendezvousRelay {
		roles = []string{"initiator", "introduction", "responder", "gateway", "alpha-gateway", "alpha-relay"}
	}
	for _, role := range roles {
		result := h43RemoteResult(t, remote, role)
		if result.Class != "drained" {
			t.Fatalf("remote H4-3B %s result class = %q, want drained", role, result.Class)
		}
	}
	if terminal != referenceC2PublisherEndpointStop {
		publisher := h43RemoteResult(t, remote, "publisher")
		if scenario.dynamicWorkload.configured() {
			assertReferenceC2EndpointRuntime(t, scenario, "publisher", publisher.Runtime)
		}
		if terminal == referenceC2PublisherWithdrawal && publisher.Class != "unpublished" {
			t.Fatalf("remote H4-3B Publisher withdrawal class = %q, want unpublished", publisher.Class)
		}
	}
	if terminal != referenceC2PublisherApplicationReset && terminal != referenceC2PublisherEndpointStop && scenario.transitFault == "" {
		if application := h43RemoteResult(t, remote, "publisher-app"); application.Class != "served" {
			t.Fatalf("remote H4-3B Publisher Application class = %q, want served", application.Class)
		}
	}
	if terminal == referenceC2PublisherApplicationReset {
		expected := "simulated Publisher Application crash after partial response"
		if scenario.dynamicWorkload.configured() {
			expected = "simulated Publisher Application crash after configured warmup"
		}
		h43AssertRemoteExpectedFailure(t, remote, "publisher-app", expected)
	}
	if terminal == referenceC2PublisherEndpointStop {
		remote.waitFile(t, remote.environment.remoteDirectory+"/publisher-crash-ready", time.Now().Add(time.Second))
		h43AssertRemoteExpectedFailure(t, remote, "publisher-app", "simulated Publisher Endpoint crash closed the local Application handoff")
	}
	if scenario.transitFault != "" {
		expected := "simulated Carrier loss closed the local Application handoff"
		if scenario.transitFault == referenceC2TransitFaultProductNodeLoss {
			expected = "simulated product Node loss closed the local Application handoff"
		}
		h43AssertRemoteExpectedFailure(t, remote, "publisher-app", expected)
	}
}

func h43AssertRemoteExpectedFailure(t *testing.T, remote h43RemoteC2, role, expected string) {
	t.Helper()
	output, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+role+".err")
	if err != nil || !strings.Contains(string(output), expected) {
		t.Fatalf("remote H4-3B %s expected failure = %q / %v, want %q", role, output, err, expected)
	}
}

type h43RoleResult struct {
	Schema, Role, Class string
	Passed              bool
	Runtime             *endpointapi.RuntimeResult
	CarrierRelay        *h48A11CarrierRelaySnapshot
}

func h43RemoteResult(t *testing.T, remote h43RemoteC2, role string) h43RoleResult {
	t.Helper()
	output, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+role+".log")
	if err != nil {
		t.Fatalf("read remote H4-3B %s result: %v", role, err)
	}
	line := strings.TrimSpace(string(output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	var result h43RoleResult
	if err := json.Unmarshal([]byte(line), &result); err != nil || result.Schema != "ardents-e2e-reference-c2-result-v1" || result.Role != role || !result.Passed || result.Class == "" {
		t.Fatalf("remote H4-3B %s result = %q / %+v / %v", role, output, result, err)
	}
	return result
}

func h43BuildLinuxCommand(t *testing.T, name, destination string) {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", destination, "./cmd/"+name)
	command.Dir = filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOARCH=amd64", "GOOS=linux", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross-build Linux %s: %v\n%s", name, err, output)
	}
}

func h43BuildLinuxFixture(t *testing.T, destination string) {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-tags", "browsercompat", "-o", destination, "./tests/e2e/service/fixturecommand/reference-c2")
	command.Dir = filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOARCH=amd64", "GOOS=linux", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross-build Linux reference C-2 fixture: %v\n%s", err, output)
	}
}

func h43LocalProductCommand(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		return ""
	}
	return buildProductCommand(t, name)
}

func h43CopyFile(t *testing.T, destination, source string) {
	t.Helper()
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	h43WriteFile(t, destination, contents, 0o600)
}

func h43WriteFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
}

func h43WriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	contents, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	h43WriteFile(t, path, contents, 0o600)
}

func h43CloneFixture(t *testing.T, fixture map[string]any) map[string]any {
	t.Helper()
	cloned := make(map[string]any, len(fixture))
	for key, value := range fixture {
		cloned[key] = value
	}
	return cloned
}

func h43Digest(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func h43Revision(t *testing.T) string {
	t.Helper()
	status, err := exec.Command("git", "status", "--porcelain=v1").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(status))) != 0 {
		t.Fatalf("H4-3B multi-host qualification requires a clean source worktree: %v\n%s", err, status)
	}
	output, err := exec.Command("git", "rev-parse", "HEAD").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(output))) != 40 {
		t.Fatalf("read H4-3B source revision: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func h43StageDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	if err := h43WriteArchive(root, digest); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func h43ScenarioName(terminal referenceC2PublisherTerminal) string {
	switch terminal {
	case "":
		return "dynamic-http"
	case referenceC2PublisherWithdrawal:
		return "publisher-withdrawal"
	case referenceC2PublisherApplicationReset:
		return "publisher-application-reset"
	case referenceC2PublisherEndpointStop:
		return "publisher-endpoint-loss"
	default:
		return "invalid"
	}
}
