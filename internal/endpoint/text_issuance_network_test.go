//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// A complete carrier episode may create sixteen Nodes (each with a five-second
// readiness bound) and then use the twenty-second joined-Service deadline.
// Keep that bounded episode inside one Permission hour instead of creating a
// valid request that expires while its matching Custody response is imported.
const textNetworkFixtureMinimumWindow = 2 * time.Minute

func textNetworkFixtureWindow(now time.Time) (time.Time, time.Time) {
	start := now
	if now.Truncate(time.Hour).Add(time.Hour).Sub(now) <= textNetworkFixtureMinimumWindow {
		start = now.Truncate(time.Hour).Add(time.Hour)
	}
	return start, start.Truncate(time.Hour).Add(time.Hour)
}

func waitTextNetworkFixtureStart(t *testing.T) {
	t.Helper()
	for {
		now := time.Now().UTC()
		start, _ := textNetworkFixtureWindow(now)
		if !start.After(now) {
			return
		}
		timer := time.NewTimer(time.Until(start))
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal("text network fixture window canceled")
		case <-timer.C:
		}
	}
}

// The qualified-worker and accepted-State seams are explicit fixtures. The
// five Node runtimes, Custody allocation, issuer keys, Endpoint stock/journal,
// source selection, forwarding and nested role TLS are production consumers.
func startTextIssuanceNetwork(t *testing.T, carrier route.CarrierProfile) (*endpoint, *textContext, *textSourceStateFixture) {
	t.Helper()
	return startTextControlNetwork(t, carrier, false)
}

func startTextControlNetwork(t *testing.T, carrier route.CarrierProfile, resolution bool) (*endpoint, *textContext, *textSourceStateFixture) {
	return startTextRoleNetwork(t, carrier, resolution, false)
}

func startTextRoleNetwork(t *testing.T, carrier route.CarrierProfile, resolution, publisher bool, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	return startTextRoleNetworkWithJoin(t, carrier, resolution, publisher, false, configure...)
}

func startTextRoleNetworkWithJoin(t *testing.T, carrier route.CarrierProfile, resolution, publisher, join bool, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	return startTextRoleNetworkWithRunner(t, carrier, resolution, publisher, join, nil, configure...)
}

// The optional runner isolates test observers; ordinary callers retain Node.Run.
func startTextRoleNetworkWithRunner(t *testing.T, carrier route.CarrierProfile, resolution, publisher, join bool, runner func(*testing.T, int, node.Config) func() error, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	t.Helper()
	waitTextNetworkFixtureStart(t)
	return startTextRoleNetworkWithReservedFixtureWindow(t, carrier, resolution, publisher, join, runner, configure...)
}

func startTextRoleNetworkWithFixtureCloseExpectation(t *testing.T, carrier route.CarrierProfile, resolution, publisher bool, expectation *textEndpointCloseExpectation, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	t.Helper()
	waitTextNetworkFixtureStart(t)
	return startTextRoleNetworkWithReservedFixtureWindowAndFixtureCloseExpectation(t, carrier, resolution, publisher, false, nil, expectation, configure...)
}

// startTextRoleNetworkWithReservedFixtureWindow is for a child test process
// whose parent selected the same two-minute Permission window before imposing
// its own timeout. It must only run the bounded carrier episode.
func startTextRoleNetworkWithReservedFixtureWindow(t *testing.T, carrier route.CarrierProfile, resolution, publisher, join bool, runner func(*testing.T, int, node.Config) func() error, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	return startTextRoleNetworkWithReservedFixtureWindowAndFixtureCloseExpectation(t, carrier, resolution, publisher, join, runner, nil, configure...)
}

func startTextRoleNetworkWithReservedFixtureWindowAndFixtureCloseExpectation(t *testing.T, carrier route.CarrierProfile, resolution, publisher, join bool, runner func(*testing.T, int, node.Config) func() error, expectation *textEndpointCloseExpectation, configure ...func(int, *node.Config)) (*endpoint, *textContext, *textSourceStateFixture) {
	t.Helper()
	endpoint, owner, source := textSourceContextFixture(t, expectation)
	count := 5
	if resolution {
		count = 7
		addTextResolutionState(source)
	}
	if publisher {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		owner = textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Administration)
		addTextIntroductionPrefixState(source)
		addTextResponderPrefixState(source)
		count = 15
	}
	if join {
		addTextDataJoinState(source)
		count = 16
	}
	reservations := make([]func(), count)
	certificates := make([]tls.Certificate, count)
	for index := range certificates {
		certificate, key := testCertificate(t, int64(301+index), fmt.Sprintf("text-issuance-%d", index))
		certificates[index] = certificate
		t.Cleanup(func() { clear(certificate.PrivateKey.(ed25519.PrivateKey)) })
		candidate := &source.snapshot.Candidates[index]
		candidate.PublicKey = key
		candidate.Family = fmt.Sprintf("text-network-family-%d", index)
		candidate.FamilyID = sha256.Sum256([]byte(candidate.Family))
		candidate.CarrierProfile = string(carrier)
		candidate.Endpoint, reservations[index] = reserveTextNetworkAddress(t, carrier)
	}
	maxima := [3]uint32{34, 34, 0}
	if resolution {
		maxima[0] = 64
	}
	if publisher {
		// Match the bounded Publisher permission used by the installed
		// qualification. The retained-set fixture spends substantially more than
		// the single-journey stock while remaining inside the real 16,384 total.
		maxima = [3]uint32{5461, 5461, 5462}
	}
	issuerRoot := prepareTextIssuancePermissionWithIdentity(t, owner, source, certificates[4].PrivateKey.(ed25519.PrivateKey), maxima)
	endpoint.closedTokenRoot = textNetworkPrivateRoot(t)
	last := 4
	if resolution {
		last = 5 // Introduction facts are explicit fixtures; no fake registration server.
	}
	if publisher {
		last = 14
	}
	if join {
		last = 15
	}
	for index := last; index >= 0; index-- {
		candidate := source.snapshot.Candidates[index]
		snapshot := source.snapshot
		snapshot.RecordPresent = true
		snapshot.NodeID, snapshot.NodePublicKey = candidate.NodeID, candidate.PublicKey
		snapshot.RecordGeneration = source.view.Nodes[index].DutyGeneration
		snapshot.RecordValidFrom, snapshot.RecordValidUntil = candidate.ValidFrom, candidate.ValidUntil
		snapshot.DeclaredFamily, snapshot.ProbeEndpoint = candidate.Family, candidate.Endpoint
		snapshot.CarrierProfile, snapshot.ProbeCapacity = string(carrier), 16
		root := textNetworkPrivateRoot(t)
		local, err := duty.Open(duty.Config{Root: root, Clock: time.Now, Create: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := local.Close(); err != nil {
			t.Fatal(err)
		}
		ready := make(chan struct{}, 1)
		config := node.Config{HostingRoot: textNetworkHostingRoot(t), NetworkID: snapshot.NetworkID, NodeID: snapshot.NodeID,
			IdentityKey: certificates[index].PrivateKey.(ed25519.PrivateKey),
			Current:     func() (node.DutyView, error) { return textNetworkDutyFixture{snapshot: snapshot}, nil },
			CurrentClosedProfile: func() (state.ClosedProfileView, bool) {
				profile, err := source.CurrentClosedProfile()
				return profile, err == nil
			},
			CurrentClosedRoute: func() (state.ClosedRouteView, error) {
				return source.CurrentClosedRoute()
			},
			// The fixture starts up to sixteen full Nodes in one process. Use the
			// lifecycle's maximum accepted cadence so those Nodes share at most one
			// process/cgroup pressure-sampling wave per second. Admission remains
			// fail-closed and startup does not wait for this READY-state ticker.
			LocalRoleStateRoot: root, PollInterval: time.Second, CheckPlacement: func() error { return nil },
			Emit: func(_ context.Context, event node.Event) error {
				if event.State == "READY" {
					select {
					case ready <- struct{}{}:
					default:
					}
				}
				return nil
			},
		}
		if index == 15 {
			config.ClosedDataJoin = node.ClosedDataJoinProfile{HostingRoot: config.HostingRoot, AdmissionRoot: textNetworkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 6 {
			config.ClosedIntroduction = node.ClosedIntroductionProfile{AdmissionRoot: textNetworkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 5 {
			config.ClosedResolution = node.ClosedResolutionProfile{Root: textNetworkPrivateRoot(t), AdmissionRoot: textNetworkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 4 {
			config.ClosedIssuer = node.ClosedIssuerProfile{Root: issuerRoot, AdmissionRoot: textNetworkPrivateRoot(t),
				Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else {
			config.ClosedForwarding = node.ClosedForwardingProfile{Root: textNetworkPrivateRoot(t), HostingRoot: config.HostingRoot,
				Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second,
				AdmissionTraffic: resource.HostingTraffic{Tx: 32 << 20, Rx: 32 << 20}, TerminationTraffic: resource.HostingTraffic{Tx: 64 << 10, Rx: 64 << 10}}
		}
		for _, apply := range configure {
			apply(index, &config)
		}
		// Hold every selected port until its listener is about to start, so
		// earlier Node activity cannot allocate a later candidate's port.
		reservations[index]()
		if runner != nil {
			stop := runner(t, index, config)
			t.Cleanup(func() {
				if err := stop(); err != nil {
					t.Error(err)
				}
			})
			continue
		}
		// Nodes are fixture infrastructure. testing.T cancels its Context before
		// Cleanup, which would race an unrelated network shutdown against the
		// Endpoint's normal cleanup. The explicit cleanup below owns each Node.
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			result, err := node.Run(ctx, config)
			if err != nil {
				err = fmt.Errorf("Node %d result %+v: %w", index, result, err)
			}
			done <- err
		}()
		t.Cleanup(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(4 * time.Second):
				t.Error("Node runtime did not join")
			}
		})
		select {
		case <-ready:
		case err := <-done:
			done <- err
			t.Fatalf("Node %d failed before READY: %v", index, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("Node %d did not become READY", index)
		}
	}
	// Registered after Node cleanup callbacks: LIFO keeps the real network
	// available until all Endpoint channels and their workers have joined.
	t.Cleanup(func() {
		checkTextFixtureEndpointClose(t, endpoint, expectation)
	})
	return endpoint, owner, source
}

func textNetworkHostingRoot(t *testing.T) string {
	return textNetworkHostingRootWithQuantity(t, 1)
}

func textNetworkHostingRootWithQuantity(t *testing.T, quantity uint64) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	root := filepath.Join(t.TempDir(), "hosting")
	policy := resource.HostingPolicy{Provider: "test fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "GiB", Quantity: quantity,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := resource.InitializeHosting(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}
func textNetworkPrivateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func reserveTextNetworkAddress(t *testing.T, carrier route.CarrierProfile) (string, func()) {
	t.Helper()
	var address string
	var closeSocket func() error
	if carrier == route.ClosedCarrierTCP {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address, closeSocket = listener.Addr().String(), listener.Close
	} else {
		socket, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address, closeSocket = socket.LocalAddr().String(), socket.Close
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if err := closeSocket(); err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(release)
	return address, release
}
