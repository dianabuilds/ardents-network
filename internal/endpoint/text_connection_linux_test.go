//go:build linux

package endpoint

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestTextConnectionRequiresSeparateAuthorityAndExactDestination(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	endpoint.network = fixtureID(221)
	publisher := admittedTextContext(t, endpoint, principal, broker.Administration)
	if _, err := publisher.openTextConnection(); err == nil {
		t.Fatal("Administration acquired Reader owner")
	}
	contextOwner := admittedTextContext(t, endpoint, principal, broker.Connection)
	owner, err := contextOwner.openTextConnection()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	foreign, err := targetlink.Encode(targetlink.Link{Network: fixtureID(222), Target: fixtureID(223)})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []connection.Request{{Destination: connection.Name, Value: "reserved"}, {Destination: connection.TargetLink, Value: "malformed"}, {Destination: connection.TargetLink, Value: foreign}} {
		if stream, err := owner.Open(t.Context(), request); err == nil || stream != nil {
			t.Fatal("invalid destination accepted")
		}
		contextOwner.mu.Lock()
		launched := contextOwner.lastJob != nil
		contextOwner.mu.Unlock()
		if launched {
			t.Fatal("invalid destination started worker")
		}
	}
}

func TestTextConnectionReportsOnlyFixedOperationCategory(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	reported := make(chan string, 1)
	owner.mu.Lock()
	owner.operationFailure = func(failure string) { reported <- failure }
	owner.mu.Unlock()
	owner.reportTextOperationFailure("service-result")
	select {
	case failure := <-reported:
		if failure != "service-result" {
			t.Fatalf("operation failure category = %q", failure)
		}
	case <-time.After(time.Second):
		t.Fatal("operation failure category was not reported")
	}
}

func TestTextConnectionJoinsCancelledInstalledStartup(t *testing.T) {
	for _, ending := range []string{"caller", "context", "owner"} {
		t.Run(ending, func(t *testing.T) {
			endpoint, principal := textContextEndpoint(t)
			endpoint.network = fixtureID(221)
			contextOwner := admittedTextContext(t, endpoint, principal, broker.Connection)
			owner, err := contextOwner.openTextConnection()
			if err != nil {
				t.Fatal(err)
			}
			release, err := endpoint.acquireTextLaunch(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			destination, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: fixtureID(223)})
			if err != nil {
				t.Fatal(err)
			}
			caller, cancel := context.WithCancel(t.Context())
			completed := make(chan error, 1)
			var helpers sync.WaitGroup
			t.Cleanup(func() {
				cancel()
				if err := owner.Close(); err != nil {
					t.Error(err)
				}
				helpers.Wait()
			})
			helpers.Go(func() {
				stream, err := owner.Open(caller, connection.Request{Destination: connection.TargetLink, Value: destination})
				if stream != nil {
					stream.Close()
				}
				completed <- err
			})
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			var job *textJobIdentity
			for job == nil {
				contextOwner.mu.Lock()
				job = contextOwner.job
				contextOwner.mu.Unlock()
				if job != nil {
					break
				}
				select {
				case err := <-completed:
					t.Fatalf("read did not reach real launch: %v", err)
				case <-deadline.C:
					t.Fatal("read did not reserve launch")
				case <-tick.C:
				}
			}
			switch ending {
			case "caller":
				cancel()
			case "context":
				if err := contextOwner.Close(); err != nil {
					t.Fatal(err)
				}
			case "owner":
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := <-completed; err == nil {
				t.Fatal("cancelled read succeeded")
			}
			helpers.Wait()
			contextOwner.mu.Lock()
			retained, finished := contextOwner.job, job.finished
			contextOwner.mu.Unlock()
			owner.mu.Lock()
			pending := owner.pending
			owner.mu.Unlock()
			if retained != nil || !finished || pending != nil {
				t.Fatal("read returned before joining launch")
			}
		})
	}
}
