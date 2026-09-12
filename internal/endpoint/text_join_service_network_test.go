//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// State acceptance and worker qualification remain explicit fixtures. Everything
// from publication through recipient-only capsule delivery, JOIN, Service TLS,
// native authentication and document exchange uses the maintained real owners.
func TestTextJoinedServiceTransfersDocumentThroughNetwork(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, publisher, destination := textJoinedNetworkFixture(t, carrier)
			readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			now := time.Now().UTC()
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			attempt, err := reader.prepareTextIntroduction(ctx, readerJob, destination, bounds)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := textdocument.NewSnapshot([]byte("authenticated document through the joined network"))
			if err != nil {
				t.Fatal(err)
			}
			completed := make(chan error, 1)
			go func() {
				accepted, err := publisher.receiveTextIntroduction(ctx, publisherJob)
				if err != nil {
					completed <- err
					return
				}
				stream, err := publisher.openTextJoinedService(ctx, publisherJob, accepted)
				if err != nil {
					completed <- err
					return
				}
				err = snapshot.Respond(stream, stream)
				if err == nil {
					err = stream.CloseInput()
				}
				if err == nil {
					result := <-stream.Done()
					if result.Class != applicationconnection.CleanClose {
						<-stream.finished
						err = errors.Join(errors.New("joined Publisher Service ended without clean close"), stream.runErr, stream.finishErr)
					}
				}
				completed <- errors.Join(err, stream.Close())
			}()
			stream, err := reader.openTextJoinedService(ctx, readerJob, attempt)
			if err != nil {
				cancel()
				t.Fatal(errors.Join(err, <-completed))
			}
			body, readErr := textdocument.Read(ctx, stream)
			closeErr := stream.Close()
			if readErr != nil || closeErr != nil {
				cancel()
			}
			publisherErr := <-completed
			if err := errors.Join(readErr, closeErr, publisherErr); err != nil || string(body) != "authenticated document through the joined network" {
				t.Fatalf("joined document %q: %v; reader native: %v; reader cleanup: %v", body, err, stream.runErr, stream.finishErr)
			}
			for _, owner := range []*textContext{reader, publisher} {
				owner.mu.Lock()
				pending := len(owner.introductionExchanges)
				owner.mu.Unlock()
				if pending != 0 {
					t.Errorf("completed Service retained %d exchanges", pending)
				}
			}
		})
	}
}

func textJoinedNetworkFixture(t *testing.T, carrier route.CarrierProfile) (*textContext, *textContext, targetlink.Link) {
	t.Helper()
	reader, publisher := textUnpublishedNetworkFixture(t, carrier)
	endpoint := publisher.endpoint
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
	return reader, publisher, targetlink.Link{Network: endpoint.network, Target: descriptor.Descriptor.Target}
}

func textUnpublishedNetworkFixture(t *testing.T, carrier route.CarrierProfile) (*textContext, *textContext) {
	t.Helper()
	return textUnpublishedNetworkWithInstance(t, carrier, func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
		_, authority, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(authority)
		return acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), network, authority, now, until)
	})
}

func textUnpublishedNetworkWithInstance(t *testing.T, carrier route.CarrierProfile, acquire func([32]byte, time.Time, time.Time) (*instance.Root, *instance.Binding)) (*textContext, *textContext) {
	t.Helper()
	endpoint, publisher, source := startTextRoleNetworkWithJoin(t, carrier, true, true, true)
	now := time.Now().UTC().Truncate(time.Second)
	root, binding := acquire(endpoint.network, now.Add(-time.Second), source.view.Profile.NotAfter)
	credential, err := root.Credential()
	if err != nil {
		t.Fatal(err)
	}
	public := ed25519.PublicKey(credential.AuthorityPublic[:])
	t.Cleanup(func() {
		if err := errors.Join(endpoint.Close(), root.Close()); err != nil {
			t.Error(err)
		}
	})
	publications, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publications, [32]byte(public)
	reader := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
	source.issuePermission(t, reader, [3]uint32{64, 64, 0})
	return reader, publisher
}
