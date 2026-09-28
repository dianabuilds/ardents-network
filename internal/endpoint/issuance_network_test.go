//go:build linux

package endpoint

import (
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
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// A complete carrier episode may create sixteen Nodes (each with a five-second
// readiness bound) and then use the twenty-second joined-Service deadline.
// Keep that bounded episode inside one Permission hour instead of creating a
// valid request that expires while its matching Custody response is imported.
const networkFixtureMinimumWindow = 2 * time.Minute

func networkFixtureWindow(now time.Time) (time.Time, time.Time) {
	start := now
	if now.Truncate(time.Hour).Add(time.Hour).Sub(now) <= networkFixtureMinimumWindow {
		start = now.Truncate(time.Hour).Add(time.Hour)
	}
	return start, start.Truncate(time.Hour).Add(time.Hour)
}

func waitNetworkFixtureStart(t *testing.T) {
	t.Helper()
	for {
		now := time.Now().UTC()
		start, _ := networkFixtureWindow(now)
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

type roleNetworkFixture struct {
	carrier    routecarrier.CarrierProfile
	resolution bool
	publisher  bool
	join       bool
	// reservedWindow is set only by a child process whose parent selected
	// the current Permission window before starting its bounded episode.
	reservedWindow bool
	runner         func(*testing.T, int, node.Config, state.Snapshot) func() error
	configure      []func(int, *node.Config)
}

// The qualified-worker and accepted-State seams are explicit fixtures. The
// selected Node runtimes, Custody allocation, issuer keys, Endpoint stock and
// journal, Source selection, forwarding, and nested role TLS remain real.
func startRoleNetwork(t *testing.T, fixture roleNetworkFixture) (*endpoint, *dutyContext, *sourceStateFixture) {
	t.Helper()
	if fixture.publisher && !fixture.resolution || fixture.join && !fixture.publisher {
		t.Fatal("text role network fixture requires resolution before publisher and publisher before JOIN")
	}
	if !fixture.reservedWindow {
		waitNetworkFixtureStart(t)
	}
	endpoint, owner, source := sourceContextFixture(t)
	count := 5
	if fixture.resolution {
		count = 7
		addResolutionState(source)
	}
	if fixture.publisher {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		owner = permissionContextFixture(t, endpoint, fixtureID(211), broker.Administration)
		addIntroductionPrefixState(source)
		addResponderPrefixState(source)
		count = 15
	}
	if fixture.join {
		addDataJoinState(source)
		count = 16
	}
	events := make([]*networkNodeEvents, count)
	// Node cleanup callbacks run first. A failed test then gets a bounded
	// chronology for every selected duty, including events after READY.
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		for index, history := range events {
			if history == nil {
				continue
			}
			for _, event := range history.snapshot() {
				t.Logf("Node %d: %s kind=%s state=%s carrier=%s reason=%s", index,
					event.at.Format(time.RFC3339Nano), event.kind, event.state, event.carrier, event.reason)
			}
		}
	})
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
		candidate.CarrierProfile = string(fixture.carrier)
		candidate.Endpoint, reservations[index] = reserveNetworkAddress(t, fixture.carrier)
	}
	maxima := [3]uint32{34, 34, 0}
	if fixture.resolution {
		maxima[0] = 64
	}
	if fixture.publisher {
		// Match the bounded Publisher permission used by the installed
		// qualification. The retained-set fixture spends substantially more than
		// the single-journey stock while remaining inside the real 16,384 total.
		maxima = [3]uint32{5461, 5461, 5462}
	}
	issuerRoot := prepareIssuancePermissionWithIdentity(t, owner, source, certificates[4].PrivateKey.(ed25519.PrivateKey), maxima)
	endpoint.closedTokenRoot = networkPrivateRoot(t)
	last := 4
	if fixture.resolution {
		last = 5 // Introduction facts are explicit fixtures; no fake registration server.
	}
	if fixture.publisher {
		last = 14
	}
	if fixture.join {
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
		snapshot.CarrierProfile, snapshot.ProbeCapacity = string(fixture.carrier), 16
		root := networkPrivateRoot(t)
		local, err := duty.Open(duty.Config{Root: root, Clock: time.Now, Create: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := local.Close(); err != nil {
			t.Fatal(err)
		}
		history := &networkNodeEvents{}
		events[index] = history
		runtime := newTextNetworkNodeRuntime(history)
		config := node.Config{HostingRoot: networkHostingRoot(t), NetworkID: snapshot.NetworkID, NodeID: snapshot.NodeID,
			IdentityKey: certificates[index].PrivateKey.(ed25519.PrivateKey),
			Current:     func() (state.NodeDuty, error) { return state.ProjectNodeDuty(snapshot), nil },
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
			LocalRoleStateRoot: root, PollInterval: time.Second, CheckPlacement: func() error { return nil }, Emit: runtime.emit,
		}
		if index == 15 {
			config.ClosedDataJoin = node.ClosedDataJoinProfile{HostingRoot: config.HostingRoot, AdmissionRoot: networkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 6 {
			config.ClosedIntroduction = node.ClosedIntroductionProfile{AdmissionRoot: networkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 5 {
			config.ClosedResolution = node.ClosedResolutionProfile{Root: networkPrivateRoot(t), AdmissionRoot: networkPrivateRoot(t), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else if index == 4 {
			config.ClosedIssuer = node.ClosedIssuerProfile{Root: issuerRoot, AdmissionRoot: networkPrivateRoot(t),
				Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}
		} else {
			config.ClosedForwarding = node.ClosedForwardingProfile{Root: networkPrivateRoot(t), HostingRoot: config.HostingRoot,
				Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second,
				AdmissionTraffic: resource.HostingTraffic{Tx: 32 << 20, Rx: 32 << 20}, TerminationTraffic: resource.HostingTraffic{Tx: 64 << 10, Rx: 64 << 10}}
		}
		for _, apply := range fixture.configure {
			apply(index, &config)
		}
		// Hold every selected port until its listener is about to start, so
		// earlier Node activity cannot allocate a later candidate's port.
		reservations[index]()
		runtime.start(t, index, config, snapshot, fixture.runner)
	}
	// Registered after Node cleanup callbacks: LIFO keeps the real network
	// available until all Endpoint channels and their workers have joined.
	t.Cleanup(func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	})
	return endpoint, owner, source
}

func networkHostingRoot(t *testing.T) string {
	return networkHostingRootWithQuantity(t, 1)
}

func networkHostingRootWithQuantity(t *testing.T, quantity uint64) string {
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
func networkPrivateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func reserveNetworkAddress(t *testing.T, carrier routecarrier.CarrierProfile) (string, func()) {
	t.Helper()
	var address string
	var closeSocket func() error
	if carrier == routecarrier.ClosedCarrierTCP {
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
