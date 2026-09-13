//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/route"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func TestTextServiceRecoveryRejectsLateAttachmentAfterJobRetirement(t *testing.T) {
	client, publisher, _ := textServiceFixture(t)
	initialClient, initialPublisher := net.Pipe()
	replacementClient, replacementPublisher := net.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	arrived := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	testOwner := newTextServiceRecoveryTestOwner(t, cancel, initialClient, initialPublisher, replacementClient, replacementPublisher)
	testOwner.retainContext(client.owner)
	testOwner.retainContext(publisher.owner)
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		testOwner.Close()
	})
	opener := func(role string, connection net.Conn) textServiceAttachmentOpener {
		return func(context.Context, nativeconnection.Recovery) (net.Conn, [32]byte, error) {
			arrived <- role
			<-release // Model a transport result that ignores cancellation and arrives late.
			return connection, fixtureID(96), nil
		}
	}

	publisherOpened := make(chan openedTextService, 1)
	testOwner.Go(func() {
		stream, err := publisher.openTextServiceStreamWithRecovery(ctx, initialPublisher, fixtureID(95),
			opener("publisher", replacementPublisher))
		testOwner.retainStream(stream)
		publisherOpened <- openedTextService{stream: stream, err: err}
	})
	clientStream, err := client.openTextServiceStreamWithRecovery(ctx, initialClient, fixtureID(95),
		opener("client", replacementClient))
	testOwner.retainStream(clientStream)
	if err != nil {
		t.Fatal(errors.Join(err, (<-publisherOpened).err))
	}
	remote := <-publisherOpened
	if remote.err != nil {
		t.Fatal(remote.err)
	}

	accepted := make(chan struct{})
	publisherDone := make(chan error, 1)
	body := bytes.Repeat([]byte("must not survive retired authority"), 2048)
	snapshot, err := textdocument.NewSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	testOwner.Go(func() {
		reader := &interruptAfterAcceptedRequest{Reader: remote.stream, accepted: accepted,
			interrupt: func() { _ = initialPublisher.Close() }}
		err := snapshot.Respond(reader, remote.stream)
		if err == nil {
			err = remote.stream.CloseInput()
		}
		if err == nil {
			outcome := <-remote.stream.Done()
			if outcome.Class == applicationconnection.CleanClose {
				err = errors.New("retired Publisher authority completed a response")
			}
		}
		publisherDone <- errors.Join(err, remote.stream.Close())
	})
	type readResult struct {
		body []byte
		err  error
	}
	readDone := make(chan readResult, 1)
	testOwner.Go(func() {
		body, err := textdocument.Read(ctx, clientStream)
		readDone <- readResult{body: body, err: err}
	})

	roles := map[string]bool{}
	for range 2 {
		select {
		case role := <-arrived:
			roles[role] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("both recovery openers did not start: %v", roles)
		}
	}
	client.owner.retireJob(client.job)
	publisher.owner.retireJob(publisher.job)
	releaseOnce.Do(func() { close(release) })

	result := <-readDone
	publisherErr := <-publisherDone
	clientCloseErr := clientStream.Close()
	if result.err == nil || publisherErr == nil || len(result.body) != 0 {
		t.Fatalf("late Attachment crossed retired authority: body=%d client=%v close=%v Publisher=%v",
			len(result.body), result.err, clientCloseErr, publisherErr)
	}
	if !roles["client"] || !roles["publisher"] {
		t.Fatalf("recovery did not reach both late callback boundaries: %v", roles)
	}
	for _, connection := range []net.Conn{replacementClient, replacementPublisher} {
		if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
			continue
		}
		var one [1]byte
		if _, err := connection.Read(one[:]); err == nil {
			t.Fatal("late replacement transport remained open")
		}
	}
}

func TestTextRecoveryPublisherRejectsCapsuleBeyondLocalAttemptDeadline(t *testing.T) {
	reader, publisher, destination := textJoinedNetworkFixture(t, route.ClosedCarrierTCP)
	readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	bounds := [3]int64{now.Add(12 * time.Second).Unix(), now.Add(12 * time.Second).Unix(), now.Add(12 * time.Second).Unix()}
	initial, err := reader.prepareTextIntroduction(ctx, readerJob, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(initial.operation)
	lease, err := publisher.endpoint.publications.AcquireAt(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	current := lease.Current()
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	publisherBinding, err := publisher.acceptTextServiceBinding(publisherJob, current, initial.binding.facts)
	if err != nil {
		t.Fatal(err)
	}

	clientRequest := initial.binding.textServiceRecovery()
	clientRequest.Generation, clientRequest.Role, clientRequest.Deadline = 2, "client", now.Add(8*time.Second).UTC().Truncate(time.Second)
	recovery, err := reader.prepareTextRecovery(ctx, readerJob, initial.binding, clientRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(recovery.operation)
	publisherRequest := publisherBinding.textServiceRecovery()
	publisherRequest.Generation, publisherRequest.Role, publisherRequest.Deadline = 2, "publisher", now.Add(2*time.Second).UTC().Truncate(time.Second)
	if !recovery.plaintext.Deadline.After(publisherRequest.Deadline) {
		t.Fatal("fixture did not exceed the Publisher recovery deadline")
	}
	if accepted, err := publisher.acceptTextRecovery(ctx, publisherJob, recovery.operation, publisherBinding, publisherRequest); err == nil || accepted != nil {
		t.Fatal("Publisher accepted a recovery capsule beyond its local attempt deadline")
	}
}
