package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestReferenceC2RunsEveryRoleInSeparateProcesses(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{})
}

func TestReferenceC2ReportsUnavailableAfterPublisherGoesOffline(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{publisherOffline: true})
}

func TestReferenceC2RejectsUntrustedPublisherApplication(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{rejectPublisherApplication: true})
}

func TestReferenceC2CarriesOneDynamicPublisherApplication(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true})
}

func TestReferenceC2ExplicitlyWithdrawsDynamicPublication(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherWithdrawal})
}

func TestReferenceC2ClassifiesPublisherApplicationCrash(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherApplicationReset})
}

func TestReferenceC2ClassifiesAbruptPublisherEndpointLoss(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherEndpointStop})
}

func TestReferenceC2CarriesOneRouteThroughProductNodeCommands(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{productNodeTransit: true})
}

func TestReferenceC2ProductNodeCommandsReportUnavailableAfterPublisherGoesOffline(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{publisherOffline: true, productNodeTransit: true})
}

func TestReferenceC2RefreshesStateAndWithdrawsProductNodeCommands(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{productNodeTransit: true, refreshWithdrawsProductNodes: true})
}

func TestReferenceC2RefreshesStateAndWithdrawsProductNodesWithHeldRoute(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{productNodeTransit: true, refreshWithdrawsHeldProductRoute: true})
}

func TestReferenceC2HardStopsRendezvousWithHeldRoute(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{productNodeTransit: true, hardStopsHeldRendezvous: true})
}

type referenceC2Scenario struct {
	publisherTerminal                                                                                                                                                                                                                             referenceC2PublisherTerminal
	transitFault                                                                                                                                                                                                                                  referenceC2TransitFault
	dynamicWorkload                                                                                                                                                                                                                               referenceC2DynamicWorkload
	publisherOffline, rejectPublisherApplication, transparentApplication, browserEntryDynamic, signedFirefox, productNodeTransit, productRendezvousRelay, refreshWithdrawsProductNodes, refreshWithdrawsHeldProductRoute, hardStopsHeldRendezvous bool
}

type referenceC2PublisherTerminal string

const (
	referenceC2PublisherWithdrawal       referenceC2PublisherTerminal = "withdrawal"
	referenceC2PublisherApplicationReset referenceC2PublisherTerminal = "application-reset"
	referenceC2PublisherEndpointStop     referenceC2PublisherTerminal = "endpoint-stop"
)

func runReferenceC2(t *testing.T, scenario referenceC2Scenario) {
	t.Helper()
	if scenario.productRendezvousRelay {
		t.Fatal("product Rendezvous relay requires the multihost runner")
	}
	switch scenario.transitFault {
	case "":
	case referenceC2TransitFaultCarrierLoss, referenceC2TransitFaultProductNodeLoss:
		t.Fatal("product transit fault requires the multihost runner")
	default:
		t.Fatalf("unknown product transit fault %q", scenario.transitFault)
	}
	nodeBinary := buildProductCommand(t, "ardents-node")
	fixtureBinary := buildE2EFixtureCommand(t, "reference-c2")
	var browserEntryQualification referenceC2BrowserEntryQualification
	if scenario.browserEntryDynamic {
		browserEntryQualification = prepareReferenceC2BrowserEntryQualification(t, scenario.signedFirefox)
	}
	var alphaControlEndpoint, alphaControlCommand string
	if runtime.GOOS == "linux" {
		alphaControlEndpoint = buildProductCommand(t, "ardents")
		alphaControlCommand = buildProductCommand(t, "ardents-control")
	}
	now := time.Now().UTC().Truncate(time.Second)
	timeBudget := 20 * time.Second
	timeBudget = scenario.dynamicWorkload.timeBudget(timeBudget)
	if scenario.browserEntryDynamic {
		timeBudget = 40 * time.Second
	}
	if scenario.signedFirefox {
		if !scenario.browserEntryDynamic {
			t.Fatal("signed Firefox qualification requires the Browser Entry dynamic flow")
		}
		timeBudget = 4 * time.Minute
	}
	if scenario.refreshWithdrawsProductNodes || scenario.refreshWithdrawsHeldProductRoute || scenario.hardStopsHeldRendezvous {
		if !scenario.productNodeTransit {
			t.Fatal("State refresh withdrawal requires product Node transit")
		}
		// The full gate runs process packages concurrently. Keep one bounded
		// budget large enough for all four product Nodes to report their State
		// transition before the harness releases an already-held Route.
		timeBudget = 60 * time.Second
	}
	if scenario.refreshWithdrawsHeldProductRoute && scenario.hardStopsHeldRendezvous {
		t.Fatal("C2 held-route State withdrawal and abrupt Rendezvous loss are mutually exclusive")
	}
	deadline := now.Add(timeBudget)
	introductionID, rendezvousID := referenceC2ID(3), referenceC2ID(4)
	responderID, initiatorID := referenceC2ID(5), referenceC2ID(6)
	gatewayID := referenceC2ID(13)
	introductionMaterial := referenceC2Certificate(t, 3, "introduction")
	rendezvousMaterial := referenceC2Certificate(t, 4, "rendezvous")
	responderMaterial := referenceC2Certificate(t, 5, "responder")
	initiatorMaterial := referenceC2Certificate(t, 6, "initiator")
	gatewayMaterial := referenceC2Certificate(t, 7, "gateway")
	addresses := referenceC2Addresses(t, 5)
	introductionAddress, rendezvousAddress := addresses[0], addresses[1]
	responderAddress, initiatorAddress, gatewayAddress := addresses[2], addresses[3], addresses[4]
	join, reachability := referenceC2ID(7), referenceC2ID(8)
	slotAttachment, serviceAttachment, resolutionAttachment := referenceC2ID(9), referenceC2ID(10), referenceC2ID(12)
	transitAuthorityPublic, transitAuthorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stateFixture := newReferenceC2StateFixture(t, now, deadline, transitAuthorityPrivate, map[string]referenceC2StateRecord{
		"introduction":           {role: "introduction", nodeID: introductionID, material: introductionMaterial, endpoint: introductionAddress, family: "reference-introduction"},
		"rendezvous":             {role: "rendezvous", nodeID: rendezvousID, material: rendezvousMaterial, endpoint: rendezvousAddress, family: "reference-rendezvous"},
		"responder":              {role: "responder", nodeID: responderID, material: responderMaterial, endpoint: responderAddress, family: "reference-responder"},
		"initiator":              {role: "initiator", nodeID: initiatorID, material: initiatorMaterial, endpoint: initiatorAddress, family: "reference-initiator"},
		"destination-resolution": {role: "destination-resolution", nodeID: gatewayID, material: gatewayMaterial, endpoint: gatewayAddress, family: "reference-gateway"},
	})
	network, digest := stateFixture.network, stateFixture.digest
	epoch := stateFixture.epoch
	slotCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, epoch, introductionID, route.IntroductionRole, slotAttachment, deadline, 31)
	responderCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, epoch, responderID, route.ResponderRole, serviceAttachment, deadline, 32)
	introductionCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, epoch, introductionID, route.IntroductionRole, serviceAttachment, deadline, 33)
	inviteID := referenceC2ID(11)

	root := t.TempDir()
	// Fixture roles use deadline for their own bounded work. Keep the parent
	// alive briefly longer so a child can report its classified failure instead
	// of being killed by the parent at the identical instant.
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(5*time.Second))
	defer cancel()
	sources := referenceC2StartStateSources(t, ctx, nodeBinary, stateFixture, root)
	sourceEndpoints, sourceClient := sources.endpoints, sources.client
	clientCertificate, err := os.ReadFile(sourceClient.certificate)
	if err != nil {
		t.Fatal(err)
	}
	clientPrivateKey, err := os.ReadFile(sourceClient.privateKey)
	if err != nil {
		t.Fatal(err)
	}
	sourceConfig := make([]map[string]string, len(sourceEndpoints))
	for index, source := range sourceEndpoints {
		sourceConfig[index] = map[string]string{"Address": source.address, "ServerName": source.serverName, "Root": source.root,
			"LeafKeyDigest": hex.EncodeToString(source.leafDigest[:])}
	}
	publicationPath := filepath.Join(root, "publication.json")
	configPath := filepath.Join(root, "reference-c2.json")
	alphaAuthorityPublic, alphaAuthorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	alphaCorpusFloorRoot := filepath.Join(root, "alpha-corpus-floor")
	alphaObserverCorpusFloorRoot := filepath.Join(root, "alpha-observer-corpus-floor")
	alphaServiceLink := "ardents-alpha://reference"
	alphaGatewayReadyPath := filepath.Join(root, "alpha-gateway-ready.json")
	alphaRelayReadyPath := filepath.Join(root, "alpha-relay-ready.json")
	readyRoot := filepath.Join(root, "ready")
	if err := os.Mkdir(readyRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	completePath := filepath.Join(root, "complete")
	heldRouteReadyPath := filepath.Join(root, "held-route-ready")
	heldRouteUserReadyPath := filepath.Join(root, "held-route-user-ready")
	heldRouteReleasePath := filepath.Join(root, "held-route-release")
	resourceProofPath := filepath.Join(root, "reference-resources")
	publisherCrashReadyPath := filepath.Join(root, "publisher-crash-ready")
	browserEntryStatePath := filepath.Join(root, "browser-entry.json")
	publisherApplicationAddress := "127.0.0.1:0"
	publisherApplicationAddressPath := filepath.Join(root, "publisher-application-address")
	publisherApplicationReady := filepath.Join(root, "publisher-application-ready")
	stateRoots := map[string]string{}
	stateMaterials := map[string]uint32{}
	var invite []byte
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		stateRoot := filepath.Join(root, role+"-state")
		candidate := referenceC2AcceptState(t, stateFixture, stateRoot, role)
		stateRoots[role] = stateRoot
		stateMaterials[role] = stateFixture.roles[role].materializationIndex
		if role == "initiator" {
			invite = referenceC2EntryInvite(t, initiatorMaterial, network, digest, epoch, candidate, deadline, now)
		}
	}
	if len(invite) == 0 {
		t.Fatal("reference C2 State fixture did not issue the Initiator Entry Invite")
	}
	fixture := map[string]any{
		"Schema": "ardents-e2e-reference-c2-v1", "Network": referenceC2Hex(network), "Digest": referenceC2Hex(digest),
		"Epoch": epoch, "Deadline": deadline.Format(time.RFC3339), "PublicationPath": publicationPath, "PublisherRoot": filepath.Join(root, "publisher-state"),
		"GatewayRoot": filepath.Join(root, "gateway-state"), "GatewayProfilePath": filepath.Join(root, "gateway-profile.json"),
		"ReadyRoot": readyRoot, "CompletePath": completePath, "ResourceProofPath": resourceProofPath,
		"PublisherApplicationAddress": publisherApplicationAddress, "PublisherApplicationAddressPath": publisherApplicationAddressPath,
		"PublisherApplicationToken": referenceC2Hex(referenceC2ID(14)), "PublisherApplicationReady": publisherApplicationReady,
		"Introduction": referenceC2Peer(introductionID, introductionMaterial, introductionAddress), "Rendezvous": referenceC2Peer(rendezvousID, rendezvousMaterial, rendezvousAddress),
		"Responder": referenceC2Peer(responderID, responderMaterial, responderAddress), "Initiator": referenceC2Peer(initiatorID, initiatorMaterial, initiatorAddress),
		"Gateway":    referenceC2Peer(gatewayID, gatewayMaterial, gatewayAddress),
		"JoinHandle": referenceC2Hex(join), "Reachability": referenceC2Hex(reachability), "SlotAttachment": referenceC2Hex(slotAttachment),
		"ServiceAttachment": referenceC2Hex(serviceAttachment), "ResolutionAttachment": referenceC2Hex(resolutionAttachment),
		"TransitAuthority": hex.EncodeToString(transitAuthorityPublic), "SlotCredential": slotCredential, "ResponderCredential": responderCredential,
		"IntroductionCredential": introductionCredential, "InviteID": referenceC2Hex(inviteID), "Invite": base64.RawStdEncoding.EncodeToString(invite), "TransitStateRoots": stateRoots, "TransitStateMaterials": stateMaterials,
		"TransitStateSources": sourceConfig, "TransitStateClient": map[string]string{"Certificate": string(clientCertificate), "PrivateKey": string(clientPrivateKey)},
		"AlphaCorpusAuthority": hex.EncodeToString(alphaAuthorityPublic), "AlphaCorpusPrivate": base64.RawStdEncoding.EncodeToString(alphaAuthorityPrivate),
		"AlphaCorpusFloorRoot": alphaCorpusFloorRoot, "AlphaObserverCorpusFloorRoot": alphaObserverCorpusFloorRoot, "AlphaServiceLink": alphaServiceLink,
		"AlphaGatewayReadyPath": alphaGatewayReadyPath, "AlphaRelayReadyPath": alphaRelayReadyPath,
		"PublisherOffline": scenario.publisherOffline, "TransparentApplication": scenario.transparentApplication,
		"PublisherTerminal": scenario.publisherTerminal, "PublisherCrashReadyPath": publisherCrashReadyPath,
	}
	scenario.dynamicWorkload.addTo(fixture)
	if scenario.browserEntryDynamic {
		fixture["BrowserEntryStatePath"] = browserEntryStatePath
	}
	if scenario.refreshWithdrawsHeldProductRoute || scenario.hardStopsHeldRendezvous {
		fixture["HeldRouteReady"] = heldRouteReadyPath
		fixture["HeldRouteUserReady"] = heldRouteUserReadyPath
		fixture["HeldRouteRelease"] = heldRouteReleasePath
	}
	if firefox := os.Getenv("ARDENTS_REFERENCE_C2_FIREFOX"); firefox != "" && !scenario.transparentApplication {
		fixture["FirefoxExecutable"] = firefox
	}
	raw, err := json.Marshal(fixture)
	if err != nil || os.WriteFile(configPath, raw, 0o600) != nil {
		t.Fatal("write process C2 fixture configuration")
	}
	publisherApplicationConfigPath := configPath
	if scenario.rejectPublisherApplication {
		invalidApplication := make(map[string]any, len(fixture))
		for key, value := range fixture {
			invalidApplication[key] = value
		}
		invalidApplication["PublisherApplicationToken"] = referenceC2Hex(referenceC2ID(15))
		publisherApplicationConfigPath = filepath.Join(root, "reference-c2-invalid-application.json")
		raw, err := json.Marshal(invalidApplication)
		if err != nil || os.WriteFile(publisherApplicationConfigPath, raw, 0o600) != nil {
			t.Fatal("write untrusted Publisher Application fixture configuration")
		}
	}
	transit := make(map[string]<-chan commandResult, 4)
	productTransit := make(map[string]*referenceC2ProductNode, 4)
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		if scenario.productNodeTransit {
			productTransit[role] = referenceC2StartProductNode(t, ctx, nodeBinary, root, stateFixture, role, stateRoots[role],
				sourceEndpoints, sourceClient)
			if err := referenceC2WaitForProductNodeReady(ctx, productTransit[role]); err != nil {
				t.Fatalf("C2 product Node %s did not become ready: %v\n%s", role, err, productTransit[role].stderr.String())
			}
		} else {
			transit[role] = startCommand(ctx, root, fixtureBinary, role, configPath)
			if err := referenceC2WaitForFile(ctx, filepath.Join(readyRoot, role)); err != nil {
				process := <-transit[role]
				t.Fatalf("C2 transit process %s did not become ready: %v\n%s", role, err, process.output)
			}
		}
	}
	if probe, err := net.DialTimeout("tcp", initiatorAddress, time.Second); err != nil {
		t.Fatalf("State-run Initiator was not listening after READY: %v", err)
	} else {
		_ = probe.Close()
	}
	gateway := startCommand(ctx, root, fixtureBinary, "gateway", configPath)
	var publisher <-chan commandResult
	var killablePublisher *killableCommand
	if scenario.publisherTerminal == referenceC2PublisherEndpointStop {
		killablePublisher = startKillableCommand(ctx, root, fixtureBinary, "publisher", configPath)
		publisher = killablePublisher.result
	} else {
		publisher = startCommand(ctx, root, fixtureBinary, "publisher", configPath)
	}
	if err := referenceC2WaitForFile(ctx, publicationPath); err != nil {
		process := <-publisher
		t.Fatalf("C2 Publisher process did not publish: %v\n%s", err, process.output)
	}
	stageReferenceC2AlphaCorpus(t, alphaControlEndpoint, alphaControlCommand, publicationPath, alphaAuthorityPublic, alphaAuthorityPrivate, alphaCorpusFloorRoot, network, alphaServiceLink)
	stageReferenceC2AlphaCorpus(t, alphaControlEndpoint, alphaControlCommand, publicationPath, alphaAuthorityPublic, alphaAuthorityPrivate, alphaObserverCorpusFloorRoot, network, alphaServiceLink)
	alphaGateway := startCommand(ctx, root, fixtureBinary, "alpha-gateway", configPath)
	if err := referenceC2WaitForFile(ctx, alphaGatewayReadyPath); err != nil {
		process := <-alphaGateway
		t.Fatalf("C2 alpha Gateway process did not become ready: %v\n%s", err, process.output)
	}
	alphaRelay := startCommand(ctx, root, fixtureBinary, "alpha-relay", configPath)
	if err := referenceC2WaitForFile(ctx, alphaRelayReadyPath); err != nil {
		process := <-alphaRelay
		t.Fatalf("C2 alpha Relay process did not become ready: %v\n%s", err, process.output)
	}
	alphaObserver := startCommand(ctx, root, fixtureBinary, "alpha-observer", configPath)
	alphaObserverResult := <-alphaObserver
	assertC2AlphaObserverResolved(t, alphaObserverResult)
	if err := referenceC2WaitForFile(ctx, filepath.Join(readyRoot, "gateway")); err != nil {
		process := <-gateway
		t.Fatalf("C2 Gateway process did not become ready: %v\n%s", err, process.output)
	}
	var publisherApplication <-chan commandResult
	if !scenario.publisherOffline {
		publisherApplication = startCommand(ctx, root, fixtureBinary, "publisher-app", publisherApplicationConfigPath)
		if scenario.rejectPublisherApplication {
			assertPublisherApplicationRejection(t, publisher, publisherApplication, publisherApplicationReady)
			if err := os.WriteFile(completePath, []byte("complete\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if scenario.productNodeTransit {
				referenceC2StopProductNodes(t, productTransit)
			} else {
				assertC2TransitDrained(t, transit, gateway)
			}
			assertC2DrainedResult(t, "alpha-gateway", <-alphaGateway)
			assertC2DrainedResult(t, "alpha-relay", <-alphaRelay)
			return
		}
		if err := referenceC2WaitForFile(ctx, publisherApplicationReady); err != nil {
			process := <-publisherApplication
			t.Fatalf("C2 Publisher local Application process did not become ready: %v\n%s", err, process.output)
		}
	}
	user := startCommand(ctx, root, fixtureBinary, "user", configPath)
	if scenario.publisherTerminal == referenceC2PublisherEndpointStop {
		if err := referenceC2WaitForFile(ctx, publisherCrashReadyPath); err != nil {
			t.Fatalf("C2 Publisher Endpoint crash point was not reached: %v", err)
		}
		if err := killablePublisher.Kill(); err != nil {
			t.Fatalf("hard-stop C2 Publisher Endpoint: %v", err)
		}
	}
	var browserEntryResult <-chan commandResult
	if scenario.browserEntryDynamic {
		if err := referenceC2WaitForFile(ctx, browserEntryStatePath); err != nil {
			process := <-user
			t.Fatalf("C2 User did not publish Browser Entry state: %v\n%s", err, process.output)
		}
		browserEntryResult = startReferenceC2BrowserEntryQualification(ctx, root, browserEntryQualification, browserEntryStatePath, resourceProofPath)
	}
	if scenario.refreshWithdrawsHeldProductRoute || scenario.hardStopsHeldRendezvous {
		if err := referenceC2WaitForFile(ctx, heldRouteReadyPath); err != nil {
			t.Fatalf("C2 product route did not become held: %v", err)
		}
		if err := referenceC2WaitForFile(ctx, heldRouteUserReadyPath); err != nil {
			t.Fatalf("C2 User did not complete held-route setup: %v", err)
		}
		if scenario.refreshWithdrawsHeldProductRoute {
			successor := referenceC2SuccessorStateFixture(t, stateFixture)
			for _, source := range sources.servers {
				source.replaceState(t, successor)
			}
			for role, process := range productTransit {
				if err := referenceC2WaitForProductNodeState(ctx, process, "DRAINING"); err != nil {
					t.Fatalf("held C2 product Node %s did not drain after refreshed State: %v\n%s", role, err, process.stderr.String())
				}
				if err := referenceC2WaitForProductNodeState(ctx, process, "WITHDRAWN"); err != nil {
					t.Fatalf("held C2 product Node %s did not withdraw after refreshed State: %v\n%s", role, err, process.stderr.String())
				}
			}
		} else {
			referenceC2HardStopProductNode(t, productTransit["rendezvous"])
		}
		if err := os.WriteFile(heldRouteReleasePath, []byte("release\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	processes := map[string]commandResult{"user": <-user}
	publisherResult := <-publisher
	if scenario.publisherTerminal == referenceC2PublisherEndpointStop {
		if publisherResult.err == nil {
			t.Fatalf("hard-stopped Publisher Endpoint reported success: %s", publisherResult.output)
		}
	} else {
		processes["publisher"] = publisherResult
	}
	if browserEntryResult != nil {
		process := <-browserEntryResult
		if process.err != nil {
			t.Fatalf("C2 dynamic Firefox Browser Entry qualification failed: %v\n%s", process.err, process.output)
		}
	}
	if publisherApplication != nil {
		applicationResult := <-publisherApplication
		assertReferenceC2PublisherApplicationCompletion(t, scenario, applicationResult, processes)
	}
	if err := os.WriteFile(completePath, []byte("complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if scenario.productNodeTransit {
		if scenario.refreshWithdrawsProductNodes {
			successor := referenceC2SuccessorStateFixture(t, stateFixture)
			for _, source := range sources.servers {
				source.replaceState(t, successor)
			}
			for role, process := range productTransit {
				if err := referenceC2WaitForProductNodeState(ctx, process, "DRAINING"); err != nil {
					t.Fatalf("C2 product Node %s did not drain after refreshed State: %v\n%s", role, err, process.stderr.String())
				}
				if err := referenceC2WaitForProductNodeState(ctx, process, "WITHDRAWN"); err != nil {
					t.Fatalf("C2 product Node %s did not withdraw after refreshed State: %v\n%s", role, err, process.stderr.String())
				}
			}
		}
		referenceC2StopProductNodes(t, productTransit)
	} else {
		for role, process := range transit {
			processes[role] = <-process
		}
	}
	processes["gateway"] = <-gateway
	processes["alpha-gateway"] = <-alphaGateway
	processes["alpha-relay"] = <-alphaRelay
	roles := []string{"user", "gateway"}
	if scenario.publisherTerminal != referenceC2PublisherEndpointStop {
		roles = append(roles, "publisher")
	}
	if !scenario.productNodeTransit {
		roles = append(roles, "initiator", "introduction", "rendezvous", "responder")
	}
	roles = append(roles, "alpha-gateway", "alpha-relay")
	if !scenario.publisherOffline && scenario.publisherTerminal != referenceC2PublisherApplicationReset && scenario.publisherTerminal != referenceC2PublisherEndpointStop {
		roles = append(roles, "publisher-app")
	}
	for _, role := range roles {
		process := processes[role]
		if process.err != nil {
			t.Fatalf("C2 %s Endpoint process failed: %v\n%s", role, process.err, process.output)
		}
		var observed struct {
			Schema, Role, Class string
			Passed              bool
		}
		line := strings.TrimSpace(string(process.output))
		if index := strings.LastIndex(line, "\n"); index >= 0 {
			line = line[index+1:]
		}
		if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" || observed.Role != role || !observed.Passed || observed.Class == "" {
			t.Fatalf("C2 Endpoint process result = %q / %+v / %v", process.output, observed, err)
		}
		if role == "rendezvous" || role == "initiator" || role == "introduction" || role == "responder" || role == "gateway" {
			if observed.Class != "drained" {
				t.Fatalf("C2 transit process %s result class = %q, want drained", role, observed.Class)
			}
		}
		if role == "user" && scenario.publisherOffline && observed.Class != "service unavailable" {
			t.Fatalf("offline C2 User result class = %q, want service unavailable", observed.Class)
		}
		if scenario.publisherTerminal == referenceC2PublisherWithdrawal && role == "publisher" && observed.Class != "unpublished" {
			t.Fatalf("withdrawn C2 Publisher result class = %q, want unpublished", observed.Class)
		}
		if scenario.publisherTerminal == referenceC2PublisherWithdrawal && role == "user" && observed.Class != "clean service connection close" {
			t.Fatalf("withdrawn C2 User result class = %q, want clean service connection close", observed.Class)
		}
		if scenario.publisherTerminal == referenceC2PublisherApplicationReset && (role == "publisher" || role == "user") && observed.Class != "abrupt connection loss" {
			t.Fatalf("Publisher Application crash C2 %s result class = %q, want abrupt connection loss", role, observed.Class)
		}
		if scenario.publisherTerminal == referenceC2PublisherEndpointStop && role == "user" && observed.Class != "abrupt connection loss" {
			t.Fatalf("Publisher Endpoint crash C2 User result class = %q, want abrupt connection loss", observed.Class)
		}
	}
	assertReferenceC2DynamicWorkloadResult(t, scenario, processes["user"])
	if publisherProcess, present := processes["publisher"]; present {
		assertReferenceC2PublisherDynamicRuntime(t, scenario, publisherProcess)
	}
}
