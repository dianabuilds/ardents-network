//go:build linux

package endpoint

import (
	"context"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestTextWorkerLifetimeRefusesUnpinnedInvocationBeforeINIT(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	attachment, peer := textAttachmentPair(t)
	// Matching local socket credentials cannot substitute for an observed and
	// pinned installed invocation. No real cgroup is created by this fixture.
	instance := textWorkerInstance{name: "ardents-text-reader@0-12-997.service", role: "reader", pid: attachment.pid, uid: attachment.uid}
	lifetime, err := initializeOwnedTextWorker(context.Background(), context.Background(), attachment, instance, job, nil, nil)
	if err == nil || lifetime != nil {
		t.Fatal("unverified invocation initialized a worker")
	}
	var one [1]byte
	if n, err := peer.Read(one[:]); n != 0 || err != io.EOF {
		t.Fatalf("INIT bytes or unclosed attachment: %d %v", n, err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("missing cgroup cleanup became successful context shutdown")
	}
	if _, err := owner.beginJob(endpoint, broker.Connection); err == nil {
		t.Fatal("unjoined invocation allowed replacement")
	}
}

func TestTextWorkerLifetimeRepeatedInitializationCannotConsumeAnotherAttachment(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := textAttachmentPair(t)
	if _, err := initializeOwnedTextWorker(context.Background(), context.Background(), first, textWorkerInstance{}, job, nil, nil); err == nil {
		t.Fatal("empty invocation accepted")
	}
	other, peer := textAttachmentPair(t)
	if _, err := initializeOwnedTextWorker(context.Background(), context.Background(), other, textWorkerInstance{}, job, nil, nil); err == nil {
		t.Fatal("job consumed twice")
	}
	if _, err := peer.Write([]byte{7}); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := io.ReadFull(other, one[:]); err != nil || one[0] != 7 {
		t.Fatalf("repeated call took ownership of another attachment: %v", err)
	}
}
