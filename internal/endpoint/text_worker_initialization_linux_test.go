//go:build linux

package endpoint

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestTextWorkerInitializationRevokeCancelsBeforeReadiness(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	attachment, peer := textAttachmentPair(t)
	instance := textWorkerInstance{name: "ardents-text-reader@0-12-997.service", role: "reader", pid: attachment.pid, uid: attachment.uid}
	finished := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		finished <- initializeTextWorker(context.Background(), attachment, instance, job, nil)
	}()
	t.Cleanup(func() {
		_ = attachment.Close()
		select {
		case <-joined:
		case <-time.After(3 * time.Second):
			t.Error("initialization did not join")
		}
	})
	var initialization [77]byte
	if _, err := io.ReadFull(peer, initialization[:]); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.admission.Revoke(principal, broker.Connection); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("revoked job accepted worker readiness")
		}
	case <-time.After(time.Second):
		t.Fatal("revoke did not interrupt readiness")
	}
	if _, err := owner.beginJob(endpoint, broker.Connection); err == nil {
		t.Fatal("revoked job was replaced before cleanup")
	}
}
