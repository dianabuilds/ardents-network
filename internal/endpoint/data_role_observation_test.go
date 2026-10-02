//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// TestTextDataJoinIsolatedRoleObservations captures the fresh DataJoin case
// separately from reader-context observations. It is P3 evidence input only:
// State and the worker remain explicit fixtures, and no host qualification or
// complete privacy verdict follows from this test.
func TestTextDataJoinIsolatedRoleObservations(t *testing.T) {
	if path := os.Getenv("ARDENTS_TEXT_ROLE_CHILD"); path != "" {
		runRoleObservationChild(t, path)
		return
	}
	if selected := os.Getenv("ARDENTS_TEXT_DATA_ROLE_CARRIER"); selected != "" {
		carrier := routecarrier.CarrierProfile(selected)
		if carrier != routecarrier.ClosedCarrierTCP && carrier != routecarrier.ClosedCarrierQUIC {
			t.Fatal("unknown data role-observation Carrier")
		}
		runDataJoinIsolatedRoleObservation(t, carrier)
		return
	}
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			runDataJoinIsolatedRoleObservationProcess(t, carrier)
		})
	}
}

func runDataJoinIsolatedRoleObservationProcess(t *testing.T, carrier routecarrier.CarrierProfile) {
	t.Helper()
	// The child has a 110-second test timeout. Select the two-minute Permission
	// window before starting it so its bounded carrier episode cannot spend its
	// own timeout waiting for the next hour.
	waitNetworkFixtureStart(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-test.run=^TestTextDataJoinIsolatedRoleObservations$", "-test.timeout=110s", "-test.v")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_TEXT_DATA_ROLE_CARRIER=" + string(carrier)}
	if root := os.Getenv("ARDENTS_TEXT_DATA_ROLE_OBSERVATIONS"); root != "" {
		command.Env = append(command.Env, "ARDENTS_TEXT_DATA_ROLE_OBSERVATIONS="+root)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated data role-observation %s: %v\n%s", carrier, err, output)
	}
}

func runDataJoinIsolatedRoleObservation(t *testing.T, carrier routecarrier.CarrierProfile) {
	output := t.TempDir()
	if root := os.Getenv("ARDENTS_TEXT_DATA_ROLE_OBSERVATIONS"); root != "" {
		var err error
		output, err = createRoleObservationOutput(root, string(carrier))
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var err error
		output, err = roleObservationCaptureRoot(output)
		if err != nil {
			t.Fatal(err)
		}
	}
	var processes []*roleProcess
	publisherRoot := networkPrivateRoot(t)
	publisherProcess := startPublisherDurableCapture(t, output, publisherRoot)
	runner := func(t *testing.T, index int, config node.Config, snapshot state.Snapshot) func() error {
		process := startRoleProcess(t, index, config, snapshot, output, "^TestTextDataJoinIsolatedRoleObservations$")
		processes = append(processes, process)
		return process.stop
	}
	endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true, join: true, runner: runner, reservedWindow: true})
	observe := func(phase string) {
		t.Helper()
		publisherProcess.capture(phase)
		for _, process := range processes {
			process.capture(phase)
			process.dump(phase)
		}
	}
	observe("startup")
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	now := time.Now().UTC().Truncate(time.Second)
	root, binding := acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), endpoint.network, authority, now.Add(-time.Second), source.view.Profile.NotAfter)
	t.Cleanup(func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	publisher, err := servicepublication.Open(servicepublication.Config{Root: publisherRoot, NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publisher, [32]byte(public)
	if _, err := owner.openPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.openIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	registration, err := owner.registerIntroduction(t.Context(), 1, time.Now().UTC().Add(90*time.Second).Truncate(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	published, err := owner.publishDescriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lookupPublishedProof(t, owner, published.Descriptor.Target), registration.CopyDescriptorLocked()) {
		t.Fatal("receiving Store proof differs")
	}
	observe("published")
	reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
	source.issuePermission(t, reader, [3]uint32{64, 64, 0})
	if _, err := reader.openPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	exchangeRouteData(t, reader, owner, source.view.Nodes[15].NodeID, func(_ []*client.ClosedJoinedStream, _ []*dutyContext) { observe("data") })
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.withdrawIntroduction(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	observe("withdrawn")
	for _, process := range processes {
		if err := process.stop(); err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"startup", "published", "data", "withdrawn"} {
			raw, err := os.ReadFile(filepath.Join(process.output, phase+".heap"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(raw, []byte("go1.7 heap dump\n")) {
				t.Fatal("invalid heap capture")
			}
		}
		verifyRoleDurableStateCapture(t, process.output, "startup", "published", "data", "withdrawn", "stopped")
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if err := publisherProcess.stop(); err != nil {
		t.Fatal(err)
	}
	verifyRoleDurableStateCapture(t, publisherProcess.output, "startup", "published", "data", "withdrawn", "stopped")
	allProcesses := append([]*roleProcess{publisherProcess}, processes...)
	if len(allProcesses) != 17 {
		t.Fatalf("captured roles = %d, want publisher plus 16 Nodes", len(allProcesses))
	}
	writeAndVerifyRoleDurableReceipt(t, output, string(carrier), allProcesses, "startup", "published", "data", "withdrawn", "stopped")
	t.Logf("fresh DataJoin observation captured 16 isolated Node roles and the Publisher durable root; incomplete P3")
}
