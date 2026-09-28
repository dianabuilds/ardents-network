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
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/node"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// State acceptance and worker qualification remain explicit fixtures. Everything
// from publication through recipient-only capsule delivery, JOIN, Service TLS,
// native authentication and document exchange uses the maintained real owners.
func TestTextJoinedServiceTransfersDocumentThroughNetwork(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, publisher, destination := joinedNetworkFixture(t, carrier)
			readerJob, publisherJob := liveCapsuleJob(t, reader), liveCapsuleJob(t, publisher)
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			now := time.Now().UTC()
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			attempt, err := reader.prepareIntroduction(ctx, readerJob, destination, bounds)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := textdocument.NewSnapshot([]byte("authenticated document through the joined network"))
			if err != nil {
				t.Fatal(err)
			}
			completed := make(chan error, 1)
			go func() {
				accepted, err := publisher.receiveIntroduction(ctx, publisherJob)
				if err != nil {
					completed <- err
					return
				}
				stream, err := publisher.openJoinedService(ctx, publisherJob, accepted)
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
						<-stream.Finished()
						err = errors.Join(errors.New("joined Publisher Service ended without clean close"), stream.RunErr(), stream.FinishErr())
					}
				}
				completed <- errors.Join(err, stream.Close())
			}()
			stream, err := reader.openJoinedService(ctx, readerJob, attempt)
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
				t.Fatalf("joined document %q: %v; reader native: %v; reader cleanup: %v", body, err, stream.RunErr(), stream.FinishErr())
			}
			for _, owner := range []*dutyContext{reader, publisher} {
				owner.mu.Lock()
				pending := introduction.ActiveExchangeCount(&owner.introduction.exchanges)
				owner.mu.Unlock()
				if pending != 0 {
					t.Errorf("completed Service retained %d exchanges", pending)
				}
			}
		})
	}
}

func joinedNetworkFixture(t *testing.T, carrier routecarrier.CarrierProfile, configure ...func(int, *node.Config)) (*dutyContext, *dutyContext, targetlink.Link) {
	return joinedNetworkFixtureWithReaderMaxima(t, carrier, [3]uint32{64, 64, 0}, configure...)
}

func joinedNetworkFixtureWithReaderMaxima(t *testing.T, carrier routecarrier.CarrierProfile, maxima [3]uint32,
	configure ...func(int, *node.Config),
) (*dutyContext, *dutyContext, targetlink.Link) {
	return joinedNetworkFixtureWithReaderMaximaAndRegistration(t, carrier, maxima, 120*time.Second, configure...)
}

func joinedNetworkFixtureWithReaderMaximaAndRegistration(t *testing.T, carrier routecarrier.CarrierProfile, maxima [3]uint32,
	registration time.Duration, configure ...func(int, *node.Config),
) (*dutyContext, *dutyContext, targetlink.Link) {
	t.Helper()
	reader, publisher := unpublishedNetworkFixtureWithReaderMaxima(t, carrier, maxima, configure...)
	endpoint := publisher.endpoint
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := publisher.openPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.openIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.registerIntroduction(t.Context(), 1, now.Add(registration)); err != nil {
		t.Fatal(err)
	}
	descriptor, err := publisher.publishDescriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return reader, publisher, targetlink.Link{Network: endpoint.network, Target: descriptor.Descriptor.Target}
}

func unpublishedNetworkFixture(t *testing.T, carrier routecarrier.CarrierProfile, configure ...func(int, *node.Config)) (*dutyContext, *dutyContext) {
	return unpublishedNetworkFixtureWithReaderMaxima(t, carrier, [3]uint32{64, 64, 0}, configure...)
}

func unpublishedNetworkFixtureWithReaderMaxima(t *testing.T, carrier routecarrier.CarrierProfile, maxima [3]uint32,
	configure ...func(int, *node.Config),
) (*dutyContext, *dutyContext) {
	t.Helper()
	endpoint, publisher, source := publisherNetworkWithInstance(t, carrier, func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
		_, authority, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(authority)
		return acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), network, authority, now, until)
	}, configure...)
	reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
	source.issuePermission(t, reader, maxima)
	return reader, publisher
}

func publisherNetworkWithInstance(t *testing.T, carrier routecarrier.CarrierProfile, acquire func([32]byte, time.Time, time.Time) (*instance.Root, *instance.Binding), configure ...func(int, *node.Config)) (*endpoint, *dutyContext, *sourceStateFixture) {
	t.Helper()
	endpoint, publisher, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true, join: true, configure: configure})
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
	publications, err := servicepublication.Open(servicepublication.Config{Root: networkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publications, [32]byte(public)
	return endpoint, publisher, source
}
