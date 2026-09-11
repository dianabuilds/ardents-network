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
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// TestTextDataJoinIsolatedRoleObservations captures the fresh DataJoin case
// separately from reader-context observations. It is P3 evidence input only:
// State and the worker remain explicit fixtures, and no host qualification or
// complete privacy verdict follows from this test.
func TestTextDataJoinIsolatedRoleObservations(t *testing.T) {
	if path := os.Getenv("ARDENTS_TEXT_ROLE_CHILD"); path != "" {
		runTextRoleObservationChild(t, path)
		return
	}
	if selected := os.Getenv("ARDENTS_TEXT_DATA_ROLE_CARRIER"); selected != "" {
		carrier := route.CarrierProfile(selected)
		if carrier != route.ClosedCarrierTCP && carrier != route.ClosedCarrierQUIC {
			t.Fatal("unknown data role-observation Carrier")
		}
		runTextDataJoinIsolatedRoleObservation(t, carrier)
		return
	}
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			runTextDataJoinIsolatedRoleObservationProcess(t, carrier)
		})
	}
}

func runTextDataJoinIsolatedRoleObservationProcess(t *testing.T, carrier route.CarrierProfile) {
	t.Helper()
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

func runTextDataJoinIsolatedRoleObservation(t *testing.T, carrier route.CarrierProfile) {
	output := t.TempDir()
	if root := os.Getenv("ARDENTS_TEXT_DATA_ROLE_OBSERVATIONS"); root != "" {
		var err error
		output, err = createTextRoleObservationOutput(root, string(carrier))
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var err error
		output, err = textRoleObservationCaptureRoot(output)
		if err != nil {
			t.Fatal(err)
		}
	}
	var processes []*textRoleProcess
	publisherRoot := textNetworkPrivateRoot(t)
	publisherProcess := startTextPublisherDurableCapture(t, output, publisherRoot)
	runner := func(t *testing.T, index int, config node.Config) func() error {
		process := startTextRoleProcess(t, index, config, output, "^TestTextDataJoinIsolatedRoleObservations$")
		processes = append(processes, process)
		return process.stop
	}
	endpoint, owner, source := startTextRoleNetworkWithRunner(t, carrier, true, true, true, runner)
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
	publisher, err := publication.Open(publication.Config{Root: publisherRoot, NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publisher, [32]byte(public)
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.openTextIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	registration, err := owner.registerTextIntroduction(t.Context(), 1, time.Now().UTC().Add(90*time.Second).Truncate(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	published, err := owner.publishTextDescriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lookupTextPublishedProof(t, owner, published.Descriptor.Target), registration.descriptor) {
		t.Fatal("receiving Store proof differs")
	}
	observe("published")
	reader := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
	source.issuePermission(t, reader, [3]uint32{64, 64, 0})
	if _, err := reader.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	exchangeTextRouteData(t, reader, owner, source.view.Nodes[15].NodeID, func() { observe("data") })
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.withdrawTextIntroduction(t.Context()); err != nil {
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
		verifyTextRoleDurableStateCapture(t, process.output, "startup", "published", "data", "withdrawn", "stopped")
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if err := publisherProcess.stop(); err != nil {
		t.Fatal(err)
	}
	verifyTextRoleDurableStateCapture(t, publisherProcess.output, "startup", "published", "data", "withdrawn", "stopped")
	allProcesses := append([]*textRoleProcess{publisherProcess}, processes...)
	if len(allProcesses) != 17 {
		t.Fatalf("captured roles = %d, want publisher plus 16 Nodes", len(allProcesses))
	}
	writeAndVerifyTextRoleDurableReceipt(t, output, string(carrier), allProcesses, "startup", "published", "data", "withdrawn", "stopped")
	t.Logf("fresh DataJoin observation captured 16 isolated Node roles and the Publisher durable root; incomplete P3")
}
