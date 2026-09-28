//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// The actual Store commits revision 1 before the barrier releases its RESULT.
// State and worker qualification are fixtures, as in the refresh counterpart.
// These are channel/revocation failures, not whole-Endpoint crash qualification.
func TestTextInitialPublicationLossBeforeAcknowledgement(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, failure := range []string{"none", "registration channel", "context revoke", "caller cancel"} {
				t.Run(failure, func(t *testing.T) {
					gate := newDescriptorACKGate()
					gate.revision = 1
					endpoint, owner, _, registered := startRegisteredPublisherNetwork(t, carrier, gate)
					t.Cleanup(gate.open)
					binding := endpoint.publisherBinding
					gate.arm(t)
					caller, cancelCaller := context.WithCancel(t.Context())
					defer cancelCaller()
					result := make(chan error, 1)
					go func() {
						_, err := owner.publishDescriptor(caller)
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
					recipient := introduction.Recipient(registered)
					premature := registered.PublishedLocked() || !introduction.PublishedAt(registered).IsZero() || owner.publication.refresh.Current() != nil
					owner.mu.Unlock()
					if premature || recipient == nil || recipient.Public(time.Now()) == [32]byte{} {
						t.Fatal("expected live initial recipient without acknowledged readiness or refresh")
					}
					switch failure {
					case "registration channel":
						if err := registered.Close(); err != nil {
							t.Fatal(err)
						}
					case "context revoke":
						owner.lease.Release()
					case "caller cancel":
						cancelCaller()
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
						ready := registered.PublishedLocked() && !introduction.PublishedAt(registered).IsZero() && owner.publication.refresh.Current() != nil
						owner.mu.Unlock()
						if outcome != nil || !ready {
							t.Fatalf("positive control did not become ready: %v", outcome)
						}
					} else {
						if outcome == nil {
							t.Fatal("lost initial registration accepted delayed ACK")
						}
						if failure == "caller cancel" && !errors.Is(outcome, context.Canceled) {
							t.Fatalf("caller cancellation was not returned after join: %v", outcome)
						}
						owner.mu.Lock()
						revived := registered.PublishedLocked() || !introduction.PublishedAt(registered).IsZero() || owner.publication.refresh.Current() != nil
						owner.mu.Unlock()
						if revived {
							t.Fatal("delayed initial ACK revived accepting readiness")
						}
						if failure == "caller cancel" {
							if _, err := owner.publishDescriptor(t.Context()); err != nil {
								t.Fatalf("cancelled caller poisoned the still-live publication owner: %v", err)
							}
						} else if _, err := owner.publishDescriptor(t.Context()); err == nil {
							t.Fatal("failed initial publication revived through exact retry")
						}
					}
					if err := owner.Close(); err != nil {
						t.Fatal(err)
					}
					if recipient.Public(time.Now()) != [32]byte{} || binding.Public() != nil {
						t.Fatal("closed initial publication retained recipient or Instance signer")
					}
					if lease, err := endpoint.publications.Acquire(t.Context()); err == nil {
						_ = lease.Close()
						t.Fatal("closed initial publication retained local availability")
					}
				})
			}
		})
	}
}

// The Store has committed the Descriptor but its RESULT remains held. A
// Source removed from the current slot cannot turn that late acknowledgement
// into readiness.
func TestTextInitialPublicationRefusesOldSourceAcknowledgement(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			gate := newDescriptorACKGate()
			gate.revision = 1
			_, owner, _, registered := startRegisteredPublisherNetwork(t, carrier, gate)
			t.Cleanup(gate.open)
			gate.arm(t)
			result := make(chan error, 1)
			go func() {
				_, err := owner.publishDescriptor(t.Context())
				result <- err
			}()
			select {
			case <-gate.held:
			case err := <-result:
				t.Fatalf("publication ended before held acknowledgement: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("publication did not reach the Store acknowledgement boundary")
			}
			owner.mu.Lock()
			old := owner.source.CurrentLocked()
			if old == nil || !owner.resolution.BusyLocked() {
				owner.mu.Unlock()
				t.Fatal("held publication did not retain its Source resolution flight")
			}
			source.TransplantLive(&owner.source, nil)
			owner.mu.Unlock()
			defer func() {
				owner.mu.Lock()
				source.TransplantLive(&owner.source, old)
				owner.mu.Unlock()
			}()
			gate.open()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("old Source acknowledgement became accepting readiness")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("old Source publication did not join after acknowledgement")
			}
			owner.mu.Lock()
			ready := registered.PublishedLocked() || owner.publication.refresh.Current() != nil
			active := owner.resolution.BusyLocked()
			owner.mu.Unlock()
			if ready || active {
				t.Fatal("old Source retained publication readiness or resolution flight")
			}
		})
	}
}
