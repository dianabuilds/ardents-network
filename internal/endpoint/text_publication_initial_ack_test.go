//go:build linux

package endpoint

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// The actual Store commits revision 1 before the barrier releases its RESULT.
// State and worker qualification are fixtures, as in the refresh counterpart.
// These are channel/revocation failures, not whole-Endpoint crash qualification.
func TestTextInitialPublicationLossBeforeAcknowledgement(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, failure := range []string{"none", "registration channel", "context revoke"} {
				t.Run(failure, func(t *testing.T) {
					gate := newTextDescriptorACKGate()
					gate.revision = 1
					closeExpectation := &textEndpointCloseExpectation{}
					endpoint, owner, _, registered := startTextRegisteredPublisherNetwork(t, carrier, gate, closeExpectation)
					t.Cleanup(gate.open)
					binding := endpoint.publisherBinding
					gate.arm(t)
					result := make(chan error, 1)
					go func() {
						_, err := owner.publishTextDescriptor(t.Context())
						result <- err
					}()
					select {
					case <-gate.held:
					case err := <-result:
						t.Fatalf("initial publication ended before actual Store commit: %v", err)
					case <-time.After(10 * time.Second):
						t.Fatal("initial Descriptor did not reach pre-ACK barrier")
					}
					owner.mu.Lock()
					recipient := registered.recipient
					premature := registered.published || !registered.publishedAt.IsZero() || owner.refresh.current() != nil
					owner.mu.Unlock()
					if premature || recipient == nil || recipient.Public(time.Now()) == [32]byte{} {
						t.Fatal("expected live initial recipient without acknowledged readiness or refresh")
					}
					switch failure {
					case "registration channel":
						if err := registered.close(); err != nil {
							t.Fatal(err)
						}
					case "context revoke":
						owner.lease.Release()
					}
					gate.open()
					var outcome error
					select {
					case outcome = <-result:
					case <-time.After(10 * time.Second):
						t.Fatal("initial publication did not join after released ACK")
					}
					if failure == "none" {
						owner.mu.Lock()
						ready := registered.published && !registered.publishedAt.IsZero() && owner.refresh.current() != nil
						owner.mu.Unlock()
						if outcome != nil || !ready {
							t.Fatalf("positive control did not become ready: %v", outcome)
						}
					} else {
						if outcome == nil {
							t.Fatal("lost initial registration accepted delayed ACK")
						}
						owner.mu.Lock()
						revived := registered.published || !registered.publishedAt.IsZero() || owner.refresh.current() != nil
						owner.mu.Unlock()
						if revived {
							t.Fatal("delayed initial ACK revived accepting readiness")
						}
						if _, err := owner.publishTextDescriptor(t.Context()); err == nil {
							t.Fatal("failed initial publication revived through exact retry")
						}
					}
					closeErr := owner.Close()
					if failure == "context revoke" && closeErr != nil {
						t.Fatalf("context revocation retained owner cleanup failure: %v", closeErr)
					} else if failure != "context revoke" && closeErr != nil {
						t.Fatal(closeErr)
					}
					if recipient.Public(time.Now()) != [32]byte{} || binding.Public() != nil {
						t.Fatal("closed initial publication retained recipient or Instance signer")
					}
					if lease, err := endpoint.publications.Acquire(t.Context()); err == nil {
						_ = lease.Close()
						t.Fatal("closed initial publication retained local availability")
					}
					if failure == "context revoke" {
						// Route may retire another retained context after this owner's
						// successful close. Classify that Endpoint failure before its
						// cleanup callbacks repeat Close.
						endpointErr := endpoint.Close()
						if endpointErr != nil {
							endpoint.textMu.Lock()
							retained := endpoint.textErr
							endpoint.textMu.Unlock()
							if retained == nil || !errors.Is(retained, route.ErrClosedSourceCleanup) || !strings.Contains(retained.Error(), "closed bootstrap child refused") {
								t.Fatalf("context revocation retained unexpected Endpoint cleanup failure: %v", endpointErr)
							}
							closeExpectation.allow(retained)
							if !closeExpectation.accepts(endpointErr) {
								t.Fatalf("context revocation Endpoint cleanup retained an additional failure: %v", endpointErr)
							}
						}
					}
				})
			}
		})
	}
}
