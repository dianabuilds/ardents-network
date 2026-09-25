//go:build linux && text_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// This installed process boundary complements the deterministic recovery
// matrix. It is one bounded smoke episode per Carrier, not NET-14 percentile or
// directional-byte qualification. The failure occurs only after the Publisher
// worker's complete 512-byte request has crossed its confined stream boundary.
func TestInstalledTextWorkersRecoverAcceptedRequestAcrossJoinedNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	if err := verifyTextEndpointService(ctx); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			exerciseInstalledTextWorkerRecovery(t, ctx, carrier)
		})
	}
}

func exerciseInstalledTextWorkerRecovery(t *testing.T, ctx context.Context, carrier route.CarrierProfile) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	reader, publisher, destination := installedTextRecoveryNetwork(t, carrier)
	body := bytes.Repeat([]byte("r"), (64<<10)-13)
	readerWorker, err := reader.launchTextWorker(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherWorker, err := publisher.launchTextWorker(ctx, body)
	if err != nil {
		_ = readerWorker.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(readerWorker.Close(), publisherWorker.Close()); err != nil {
			t.Error(err)
		}
	})
	readerInstance := installedTextWorkerInstance(t, ctx, readerWorker, "reader")
	publisherInstance := installedTextWorkerInstance(t, ctx, publisherWorker, "publisher")
	readerEvents, err := pinTextWorkerCgroup(readerInstance)
	if err != nil {
		t.Fatal(err)
	}
	publisherEvents, err := pinTextWorkerCgroup(publisherInstance)
	if err != nil {
		_ = readerEvents.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(readerEvents.Close(), publisherEvents.Close()); err != nil {
			t.Error(err)
		}
	})

	readerLifetime, finishReader, err := readerWorker.beginOperation(ctx, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	readerFinished := false
	defer func() {
		if !readerFinished {
			finishReader()
		}
	}()
	now := time.Now().UTC()
	bounds := [3]int64{now.Add(25 * time.Second).Unix(), now.Add(25 * time.Second).Unix(), now.Add(25 * time.Second).Unix()}
	prepared, err := reader.prepareTextIntroduction(readerLifetime, readerWorker.job, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(prepared.operation)

	type publisherOpening struct {
		recovery *observedTextServiceOpener
		digest   [32]byte
		err      error
	}
	opened := make(chan publisherOpening, 1)
	publisherDone := make(chan error, 1)
	releaseDelivery := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseDelivery) }) }
	t.Cleanup(release)
	requestAccepted := make(chan struct{})
	interruptedAt := make(chan time.Time, 1)

	go func() {
		publisherDone <- publisherWorker.serveFrom(ctx, func(lifetime context.Context, delivered chan<- connection.Stream) error {
			accepted, err := publisher.receiveTextIntroduction(lifetime, publisherWorker.job)
			if err != nil {
				opened <- publisherOpening{err: err}
				return err
			}
			raw, err := publisher.openTextJoinedTransport(lifetime, publisherWorker.job, accepted)
			if err != nil {
				opened <- publisherOpening{err: err}
				return err
			}
			recovery := &observedTextServiceOpener{open: publisher.textServiceRouteRecoveryOpener(publisherWorker.job, accepted.binding)}
			stream, err := accepted.binding.openTextServiceStreamWithRecovery(lifetime, raw, accepted.digest, recovery.openObserved)
			if err != nil {
				opened <- publisherOpening{err: err}
				return err
			}
			failed := &interruptAfterAcceptedRequest{Reader: stream, accepted: requestAccepted, interrupt: func() {
				interruptedAt <- time.Now()
				_ = raw.Close()
			}}
			wrapped := &installedRecoveryStream{Stream: stream, reader: failed}
			opened <- publisherOpening{recovery: recovery, digest: accepted.digest}
			select {
			case <-releaseDelivery:
			case <-lifetime.Done():
				return errors.Join(lifetime.Err(), stream.Close())
			}
			select {
			case delivered <- wrapped:
				return nil
			case <-lifetime.Done():
				return errors.Join(lifetime.Err(), stream.Close())
			}
		})
	}()

	readerRaw, err := reader.openTextJoinedTransport(readerLifetime, readerWorker.job, prepared)
	if err != nil {
		release()
		cancel()
		t.Fatalf("initial reader Route: %v; Publisher: %v", err, <-publisherDone)
	}
	clientRecovery := &observedTextServiceOpener{open: reader.textServiceRouteRecoveryOpener(readerWorker.job, prepared.binding)}
	clientStream, err := prepared.binding.openTextServiceStreamWithRecovery(readerLifetime, readerRaw, prepared.digest, clientRecovery.openObserved)
	remote := <-opened
	if err != nil || remote.err != nil {
		release()
		_ = readerRaw.Close()
		cancel()
		t.Fatalf("initial protected Service: %v; Publisher: %v; worker: %v", err, remote.err, <-publisherDone)
	}
	initialTokens := textTokenAttemptSnapshot(t, reader.endpoint)
	release()
	actual, readErr := readerWorker.completeServiceRead(ctx, readerLifetime, finishReader, clientStream, nil)
	readerFinished = true
	publisherErr := <-publisherDone
	if err := errors.Join(readErr, publisherErr); err != nil {
		t.Fatalf("installed recovery failed: %v; client=%v Publisher=%v", err, clientRecovery.outcome(), remote.recovery.outcome())
	}
	if !bytes.Equal(actual, body) {
		t.Fatalf("installed recovered document length=%d, want=%d", len(actual), len(body))
	}
	select {
	case <-requestAccepted:
	default:
		t.Fatal("installed Publisher did not accept the original request before interruption")
	}
	interruption := <-interruptedAt
	recoveryElapsed := time.Since(interruption)
	if recoveryElapsed > 5*time.Second {
		t.Fatalf("single installed recovery elapsed=%s, exceeds 5s smoke bound", recoveryElapsed)
	}
	assertInstalledRecoveryRoute(t, prepared.digest, remote.digest, clientRecovery, remote.recovery)
	assertFreshRecoveryTokenAttempts(t, initialTokens, textTokenAttemptSnapshot(t, reader.endpoint))
	for _, owner := range []*textContext{reader, publisher} {
		owner.mu.Lock()
		pending := len(owner.introductionExchanges.active)
		owner.mu.Unlock()
		if pending != 0 {
			t.Errorf("installed recovery retained %d Introduction exchanges", pending)
		}
	}
	assertInstalledTextWorkerRetired(t, ctx, readerWorker, readerInstance, readerEvents)
	assertInstalledTextWorkerRetired(t, ctx, publisherWorker, publisherInstance, publisherEvents)
	bitrate := float64(len(body)*8) / recoveryElapsed.Seconds()
	t.Logf("installed recovery smoke: useful-bytes=%d elapsed=%s useful-bitrate=%.0f-bit/s", len(body), recoveryElapsed, bitrate)
}

type installedRecoveryStream struct {
	connection.Stream
	reader io.Reader
}

func (stream *installedRecoveryStream) Read(body []byte) (int, error) {
	return stream.reader.Read(body)
}

func assertInstalledRecoveryRoute(t *testing.T, initialClient, initialPublisher [32]byte,
	client, publisher *observedTextServiceOpener) {
	t.Helper()
	clientAttempts, clientDigests, clientErr := client.observation()
	publisherAttempts, publisherDigests, publisherErr := publisher.observation()
	if clientAttempts < 1 || clientAttempts > 2 || publisherAttempts < 1 || publisherAttempts > 2 ||
		len(clientDigests) != 1 || len(publisherDigests) != 1 ||
		clientErr != nil && !errors.Is(clientErr, context.Canceled) ||
		publisherErr != nil && !errors.Is(publisherErr, context.Canceled) ||
		clientDigests[0] != publisherDigests[0] ||
		clientDigests[0] == initialClient || publisherDigests[0] == initialPublisher {
		t.Fatalf("installed recovery did not use one matching fresh Route: client=%v Publisher=%v", client.outcome(), publisher.outcome())
	}
}

func assertInstalledTextWorkerRetired(t *testing.T, ctx context.Context, worker *qualifiedTextWorker,
	instance textWorkerInstance, events *os.File) {
	t.Helper()
	removed, populated, err := readTextWorkerCgroup(events)
	if err != nil || !removed && populated {
		t.Fatalf("installed %s worker cleanup returned before cgroup retirement: removed=%t populated=%t error=%v", instance.role, removed, populated, err)
	}
	if worker.grant.Active() != 0 || worker.lease.Context().Err() == nil {
		t.Fatalf("installed %s worker authority survived recovery completion", instance.role)
	}
	requireInstalledTextWorkerCollected(t, ctx, instance.name, instance.role)
}

func installedTextRecoveryNetwork(t *testing.T, carrier route.CarrierProfile) (*textContext, *textContext, targetlink.Link) {
	t.Helper()
	reader, publisher := textUnpublishedNetworkWithInstance(t, carrier,
		func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
			return acquireInstalledServiceInstance(t, network, now, until)
		})
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := publisher.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.openTextIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.registerTextIntroduction(t.Context(), 1, now.Add(120*time.Second)); err != nil {
		t.Fatal(err)
	}
	descriptor, err := publisher.publishTextDescriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return reader, publisher, targetlink.Link{Network: publisher.endpoint.network, Target: descriptor.Descriptor.Target}
}
