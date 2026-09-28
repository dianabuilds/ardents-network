//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// This fixture replaces only installed launch/cgroup observation with a
// same-process Unix attachment and joined goroutine. It grants no host verdict.
// Both fixed worker protocols, private worker Grant leases and Service owners
// below execute production code.
func serviceWorkerFixture(t *testing.T, binding *serviceBinding, snapshot []byte) *qualifiedWorker {
	t.Helper()
	attachment, peer := textAttachmentPair(t)
	ctx, cancel := context.WithCancel(binding.job.context)
	lifetime := &workerLifetime{context: ctx, cancel: cancel, attachment: attachment, done: make(chan struct{})}
	mode := textdocument.ReaderWorker
	if binding.owner.surface == broker.Administration {
		mode = textdocument.PublisherWorker
	}
	workerDone := make(chan error, 1)
	go func() { workerDone <- textdocument.RunWorker(ctx, peer, mode) }()
	go func() {
		<-ctx.Done()
		binding.owner.retireJob(binding.job)
		_ = attachment.Close()
		lifetime.useMu.Lock()
		lifetime.closing = true
		operationDone := lifetime.operationDone
		lifetime.useMu.Unlock()
		if operationDone != nil {
			<-operationDone
		}
		// EOF/cancellation is expected when the Endpoint retires the worker.
		// Completion of this actual worker goroutine is the fixture's join.
		<-workerDone
		lifetime.err = binding.owner.finishJobCleanup(binding.job, nil)
		close(lifetime.done)
	}()
	t.Cleanup(func() {
		if err := lifetime.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := textdocument.InitializeWorker(t.Context(), attachment, mode, binding.job.nonce, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := attachment.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	binding.job.workerGrant.Close()
	binding.job.workerGrant = nil
	worker, err := binding.owner.bindWorker(binding.job, lifetime)
	if err != nil {
		t.Fatal(err)
	}
	return worker
}

func TestTextServiceQualifiedWorkersUseActualEndpointStreams(t *testing.T) {
	client, publisher, _ := serviceFixture(t)
	body := bytes.Repeat([]byte("text through both worker protocols\n"), 2048)
	reader := serviceWorkerFixture(t, client, nil)
	host := serviceWorkerFixture(t, publisher, body)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	incoming := make(chan *serviceStream)
	hostDone := make(chan error, 1)
	go func() { hostDone <- host.serve(ctx, incoming) }()
	local, remote := net.Pipe()
	opened := make(chan error, 1)
	go func() {
		stream, err := publisher.openServiceStreamWithRecovery(ctx, remote, fixtureID(93), nil)
		if err == nil {
			select {
			case incoming <- stream:
			case <-ctx.Done():
				err = errors.Join(ctx.Err(), stream.Close())
			}
		}
		close(incoming)
		opened <- err
	}()
	actual, readErr := reader.readService(ctx, client, local, fixtureID(93))
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	hostErr := <-hostDone
	if readErr != nil || hostErr != nil || !bytes.Equal(actual, body) {
		t.Fatalf("reader=%v Publisher=%v actual=%d expected=%d", readErr, hostErr, len(actual), len(body))
	}
	if !reader.completedCurrent() || !host.completedCurrent() {
		t.Fatal("successful body preceded joined current worker cleanup")
	}
	if err := client.current(); err == nil {
		t.Fatal("completed worker retained Service admission")
	}
}

func TestTextServicePublisherRefusesAnotherJobsStream(t *testing.T) {
	client, publicationOwner, _ := serviceFixture(t)
	_, otherPublisher, _ := serviceFixture(t)
	host := serviceWorkerFixture(t, otherPublisher, []byte("document of another Publisher context"))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	incoming := make(chan *serviceStream)
	hostDone := make(chan error, 1)
	go func() { hostDone <- host.serve(ctx, incoming) }()
	left, right := net.Pipe()
	opened := make(chan error, 1)
	go func() {
		stream, err := publicationOwner.openServiceStreamWithRecovery(ctx, right, fixtureID(95), nil)
		if err == nil {
			select {
			case incoming <- stream:
			case <-ctx.Done():
				err = errors.Join(ctx.Err(), stream.Close())
			}
		}
		close(incoming)
		opened <- err
	}()
	stream, err := client.openServiceStreamWithRecovery(ctx, left, fixtureID(95), nil)
	if err != nil {
		cancel()
		t.Fatalf("client setup: %v; Publisher setup: %v; worker: %v", err, <-opened, <-hostDone)
	}
	body, readErr := textdocument.Read(ctx, stream)
	openErr, hostErr := <-opened, <-hostDone
	if openErr != nil {
		t.Fatal(openErr)
	}
	if readErr == nil || len(body) != 0 || hostErr == nil {
		t.Fatal("foreign worker returned a document through another Service identity")
	}
}
