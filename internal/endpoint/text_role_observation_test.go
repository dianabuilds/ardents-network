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

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Same-run role memory, with a real Publisher/issuer/Store exchange. This is
// not installed Endpoint qualification, authenticated Source, transient-state coverage,
// or complete P3 analysis. Public State and worker qualification are fixtures.
func TestTextPublicationIsolatedRoleObservations(t *testing.T) {
	if path := os.Getenv("ARDENTS_TEXT_READER_CHILD"); path != "" {
		runTextReaderObservationChild(t, path)
		return
	}
	if path := os.Getenv("ARDENTS_TEXT_ROLE_CHILD"); path != "" {
		runTextRoleObservationChild(t, path)
		return
	}
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			output := t.TempDir()
			if root := os.Getenv("ARDENTS_TEXT_ROLE_OBSERVATIONS"); root != "" {
				if !filepath.IsAbs(root) {
					t.Fatal("capture root must be absolute")
				}
				var err error
				output, err = os.MkdirTemp(root, string(carrier)+"-")
				if err != nil {
					t.Fatal(err)
				}
			}
			var processes []*textRoleProcess
			runner := func(t *testing.T, index int, config node.Config) func() error {
				process := startTextRoleProcess(t, index, config, output)
				processes = append(processes, process)
				return process.stop
			}
			endpoint, owner, source := startTextRoleNetworkWithRunner(t, carrier, true, true, false, runner)
			observe := func(phase string) {
				t.Helper()
				for _, process := range processes {
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
			publisher, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
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
			observeTextIndependentReader(t, source, published.Descriptor.Target, registration.descriptor, output)
			observe("published")
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
			t.Logf("%d isolated Node roles captured in one publication/lookup/withdrawal run; incomplete P3", len(processes))
		})
	}
}
