//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Same-run role memory, with a real Publisher/issuer/Store exchange. This is
// not installed Endpoint qualification, authenticated Source, transient-state coverage,
// or complete P3 analysis. Public State and worker qualification are fixtures.
func TestTextPublicationIsolatedRoleObservations(t *testing.T) {
	if path := os.Getenv("ARDENTS_TEXT_READER_CHILD"); path != "" {
		runReaderObservationChild(t, path)
		return
	}
	if path := os.Getenv("ARDENTS_TEXT_ROLE_CHILD"); path != "" {
		runRoleObservationChild(t, path)
		return
	}
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			output := t.TempDir()
			if root := os.Getenv("ARDENTS_TEXT_ROLE_OBSERVATIONS"); root != "" {
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
			publisherRoot := textNetworkPrivateRoot(t)
			publisherProcess := startPublisherDurableCapture(t, output, publisherRoot)
			runner := func(t *testing.T, index int, config node.Config, snapshot state.Snapshot) func() error {
				process := startRoleProcess(t, index, config, snapshot, output)
				processes = append(processes, process)
				return process.stop
			}
			endpoint, owner, source := startTextRoleNetwork(t, textRoleNetworkFixture{carrier: carrier, resolution: true, publisher: true, runner: runner})
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
			if !bytes.Equal(lookupPublishedProof(t, owner, published.Descriptor.Target), registration.descriptor) {
				t.Fatal("receiving Store proof differs")
			}
			observeIndependentReader(t, source, published.Descriptor.Target, registration.descriptor, output, string(carrier))
			awaitReaderBootstrapRetirement(t)
			contextEvidence, foreignPermission := observeIndependentReaderContexts(t, source, published.Descriptor.Target, registration.descriptor, output)
			observe("published")
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
				if process.heap {
					for _, phase := range []string{"startup", "published", "withdrawn"} {
						raw, err := os.ReadFile(filepath.Join(process.output, phase+".heap"))
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.HasPrefix(raw, []byte("go1.7 heap dump\n")) {
							t.Fatal("invalid heap capture")
						}
					}
				}
				verifyRoleDurableStateCapture(t, process.output, "startup", "published", "withdrawn", "stopped")
			}
			if err := endpoint.Close(); err != nil {
				t.Fatal(err)
			}
			if err := publisherProcess.stop(); err != nil {
				t.Fatal(err)
			}
			verifyRoleDurableStateCapture(t, publisherProcess.output, "startup", "published", "withdrawn", "stopped")
			allProcesses := append([]*roleProcess{publisherProcess}, processes...)
			writeAndVerifyRoleDurableReceipt(t, output, string(carrier), allProcesses)
			writeAndVerifyReaderContextIsolation(t, output, string(carrier), contextEvidence, foreignPermission)
			t.Logf("15 isolated Node roles and the Publisher durable root captured in one publication/lookup/withdrawal run; incomplete P3")
		})
	}
}
