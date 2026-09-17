//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestTextIntroductionDeliversFourConcurrentReaders(t *testing.T) {
	endpoint, publisher, source := textPublisherNetworkWithInstance(t, route.ClosedCarrierTCP, func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
		_, authority, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(authority)
		return acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), network, authority, now, until)
	}, func(_ int, config *node.Config) {
		if config.ClosedForwarding.Root != "" {
			config.ClosedForwarding.ConnectionLimit = 16
		}
		if config.ClosedIssuer.Root != "" {
			config.ClosedIssuer.ConnectionLimit = 16
		}
		if config.ClosedResolution.Root != "" {
			config.ClosedResolution.ConnectionLimit = 16
		}
		if config.ClosedIntroduction.AdmissionRoot != "" {
			config.ClosedIntroduction.ConnectionLimit = 16
		}
		if config.ClosedDataJoin.AdmissionRoot != "" {
			config.ClosedDataJoin.ConnectionLimit = 16
		}
	})
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := publisher.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.openTextIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.openTextPublisherPrefix(t.Context(), &publisher.responder, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.registerTextIntroduction(t.Context(), 1, now.Add(120*time.Second)); err != nil {
		t.Fatal(err)
	}
	descriptor, err := publisher.publishTextDescriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	destination := targetlink.Link{Network: endpoint.network, Target: descriptor.Descriptor.Target}
	// The installed runner performs offline permission exchange between
	// Publisher readiness and Reader startup. Preserve that duty-wide bootstrap
	// output refill interval without pacing the concurrent Reader trigger.
	time.Sleep(10 * time.Second)
	readers := make([]*textContext, 0, 4)
	for range 4 {
		readers = append(readers, independentTextReaderFixture(t, publisher.endpoint.network, source))
	}
	// Keep bootstrap issuance outside the concurrent delivery trigger. This
	// test isolates the Introduction failure observed after worker readiness.
	for index, reader := range readers {
		if _, err := reader.openTextPrefix(t.Context()); err != nil {
			t.Fatalf("reader %d prefix: %v", index, err)
		}
		if index+1 < len(readers) {
			time.Sleep(5 * time.Second)
		}
	}

	publisherJob := liveTextCapsuleJob(t, publisher)
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Second)
	defer cancel()
	type readerWork struct {
		owner *textContext
		job   *textJobIdentity
	}
	work := make([]readerWork, len(readers))
	for index, reader := range readers {
		work[index] = readerWork{owner: reader, job: liveTextCapsuleJob(t, reader)}
	}
	start := make(chan struct{})
	senders := make(chan error, len(work))
	receivers := make(chan error, len(work))
	for range work {
		go func() {
			<-start
			_, err := publisher.receiveTextIntroduction(ctx, publisherJob)
			receivers <- err
		}()
	}
	for _, item := range work {
		go func(item readerWork) {
			<-start
			now := time.Now().UTC()
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			attempt, err := item.owner.prepareTextIntroduction(ctx, item.job, destination, bounds)
			if err == nil {
				err = item.owner.submitTextIntroduction(ctx, item.job, attempt)
			}
			senders <- err
		}(item)
	}
	close(start)
	var outcome error
	for range work {
		outcome = errors.Join(outcome, <-senders, <-receivers)
	}
	if outcome != nil {
		t.Fatalf("four concurrent Introduction deliveries: %v", outcome)
	}
}

func independentTextReaderFixture(t *testing.T, network [32]byte, source *textSourceStateFixture) *textContext {
	t.Helper()
	endpoint, principal := textContextEndpoint(t)
	endpoint.clock, endpoint.network, endpoint.closedState = time.Now, network, source
	endpoint.closedEntryRoot, endpoint.closedRoleRoot, endpoint.closedTokenRoot = t.TempDir(), t.TempDir(), textNetworkPrivateRoot(t)
	for _, root := range []string{endpoint.closedEntryRoot, endpoint.closedRoleRoot} {
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	roles, err := duty.Open(duty.Config{Root: endpoint.closedRoleRoot, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	})
	reader := textPermissionContextFixture(t, endpoint, principal, broker.Connection)
	source.issuePermission(t, reader, [3]uint32{64, 64, 0})
	return reader
}
