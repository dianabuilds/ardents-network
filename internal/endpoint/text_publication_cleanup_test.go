//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func textPublicationCommitFixture(t *testing.T) (*endpoint, *textContext, *instance.Binding, string, string, time.Time) {
	t.Helper()
	endpoint, principal := textContextEndpoint(t)
	endpoint.network = fixtureID(201)
	owner := admittedTextContext(t, endpoint, principal, broker.Administration)
	now := time.Now().UTC().Truncate(time.Second)
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	instancePath := serviceInstanceFixtureRoot(t)
	root, binding := acceptedInstanceBinding(t, instancePath, endpoint.network, authority, now.Add(-time.Second), now.Add(time.Hour))
	t.Cleanup(func() { _ = endpoint.Close(); _ = root.Close() })
	publicationPath := textNetworkPrivateRoot(t)
	publisher, err := publication.Open(publication.Config{Root: publicationPath, NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.authority, endpoint.publisherBinding, endpoint.publications = [32]byte(public), binding, publisher
	return endpoint, owner, binding, instancePath, publicationPath, now
}

func TestTextPublicationCancelledCommitRetainsCleanupOwner(t *testing.T) {
	endpoint, owner, binding, _, _, now := textPublicationCommitFixture(t)
	// Enter the exact post-commit/pre-Acquire boundary with a real persisted
	// Publication and consumed Instance; the commit transcript is a seam fixture.
	if _, err := endpoint.publications.Publish(t.Context(), publication.PublishInput{Credential: binding.Credential(), InstanceSigner: binding,
		Acknowledgement: []byte("completed publication commit boundary"), At: now}); err != nil {
		t.Fatal(err)
	}
	if err := binding.CommitPublished(1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	endpoint.publisherMu.Lock()
	lease, err := owner.finishTextPublicationCommit(ctx, nil, binding, now, nil)
	endpoint.publisherMu.Unlock()
	if lease != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled handover: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if lease, err := endpoint.publications.Acquire(t.Context()); err == nil {
		_ = lease.Close()
		t.Fatal("cancelled commit left live publication")
	}
	if binding.Public() != nil {
		t.Fatal("cancelled commit left live signer")
	}
}

func TestTextPublicationFailedWithdrawalRetainsBindingAndError(t *testing.T) {
	endpoint, owner, binding, instancePath, publicationPath, now := textPublicationCommitFixture(t)
	// Obstruct actual atomic replacement at both owners without deleting their
	// original bytes. The exact files are restored before retrying retained cleanup.
	instanceFile := filepath.Join(instancePath, "instance-root.json")
	backup := instanceFile + ".test-backup"
	if err := os.Rename(instanceFile, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(instanceFile, 0700); err != nil {
		t.Fatal(err)
	}
	floor := filepath.Join(publicationPath, "floor")
	if err := os.Mkdir(floor, 0700); err != nil {
		t.Fatal(err)
	}
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		if err := os.Remove(instanceFile); err != nil {
			t.Error(err)
		}
		if err := os.Rename(backup, instanceFile); err != nil {
			t.Error(err)
		}
		if err := os.Remove(floor); err != nil {
			t.Error(err)
		}

	}
	defer restore()
	endpoint.publisherMu.Lock()
	_, err := owner.acquireTextPublication(t.Context(), &textIntroductionRegistration{cancel: func() {}}, binding, now)
	retained := endpoint.publisherBinding == binding && endpoint.textPublisherOwner == owner
	endpoint.publisherMu.Unlock()
	if err == nil || !retained || endpoint.textAvailable() {
		t.Fatalf("failed cleanup lost owner or admission remained live: retained=%t err=%v", retained, err)
	}
	restore()
	if err := owner.retireTextPublication(); err != nil {
		t.Fatalf("retained cleanup could not complete: %v", err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("later cleanup erased original failure")
	}
	if binding.Public() != nil {
		t.Fatal("retained withdrawal left signer live")
	}
}
