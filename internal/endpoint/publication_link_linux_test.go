//go:build linux

package endpoint

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	applicationadministration "github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// State/Instance and installed-worker observation remain explicit fixtures.
// Registration, Descriptor ACK and the public local Link query are real.
func TestTextPublicationLinkRequiresCommittedLiveRun(t *testing.T) {
	_, publisher := unpublishedNetworkFixture(t, carrier.ClosedCarrierTCP)
	owner, err := publisher.openAdministration()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(t.TempDir(), "admin.sock")
	server, err := applicationadministration.Listen(socket, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := applicationadministration.RequestPublishedLink(t.Context(), socket); err == nil {
		t.Fatal("unpublished context supplied a Link")
	}
	job := liveCapsuleJob(t, publisher)
	worker := serviceWorkerFixture(t, &serviceBinding{owner: publisher, job: job}, []byte("published Link"))
	startup, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	run, err := worker.startPublication(startup)
	if err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	owner.run = run
	owner.mu.Unlock()
	encoded, err := applicationadministration.RequestPublishedLink(t.Context(), socket)
	if err != nil {
		t.Fatal(err)
	}
	link, err := targetlink.Decode(encoded)
	if err != nil || link != run.link {
		t.Fatalf("public Link changed committed destination: %v", err)
	}
	if err := owner.Withdraw(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := applicationadministration.RequestPublishedLink(t.Context(), socket); err == nil {
		t.Fatal("withdrawn publication supplied a live Link")
	}
}
