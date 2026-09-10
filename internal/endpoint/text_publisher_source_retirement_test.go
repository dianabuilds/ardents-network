//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestTextPublisherAcceptsIntroductionAfterSourceRetirement(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, publisher, destination := textJoinedNetworkFixture(t, carrier)
			readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
			until := time.Now().Add(time.Minute).Unix()
			prepared, err := reader.prepareTextIntroduction(t.Context(), readerJob, destination, [3]int64{until, until, until})
			if err != nil {
				t.Fatal(err)
			}
			defer clear(prepared.operation)
			publisher.mu.Lock()
			prefix, registration, permission := publisher.prefix, publisher.registration, publisher.permission
			reserved := permission.reserved
			publisher.mu.Unlock()
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-registration.channel.Done():
				t.Fatal("Source retirement closed independent Introduction registration")
			default:
			}
			// A validly sealed but wrong Target/Rendezvous must still refuse before
			// Source or Responder work, including when the prior Source is retired.
			_, capsule, err := route.DecodeClosedIntroductionSubmission(prepared.operation)
			if err != nil {
				t.Fatal(err)
			}
			for index := range 2 {
				wrong := prepared.plaintext
				if index == 0 {
					wrong.RendezvousNode = fixtureID(231)
				} else {
					wrong.Target = fixtureID(232)
				}
				envelope := route.ClosedIntroductionCapsule{Slot: capsule.Slot, Revision: capsule.Revision, Expiry: capsule.Expiry, DeliveryNonce: fixtureID(byte(233 + index))}
				sealed, _, err := route.SealClosedIntroduction(envelope, registration.recipient.Public(time.Now()), wrong)
				if err != nil {
					t.Fatal(err)
				}
				operation, err := route.EncodeClosedIntroductionSubmission(fixtureID(byte(235+index)), sealed)
				if err != nil {
					t.Fatal(err)
				}
				rejected, refusal := publisher.acceptTextIntroduction(t.Context(), publisherJob, operation)
				clear(operation)
				if refusal == nil || rejected != nil {
					t.Fatal("idle Publisher accepted foreign recipient facts")
				}
				publisher.mu.Lock()
				noWork := publisher.prefix == nil && publisher.responder.prefix == nil && permission.reserved == reserved
				publisher.mu.Unlock()
				if !noWork {
					t.Fatal("refused capsule created Source work or consumed allocation")
				}
			}
			accepted, err := publisher.acceptTextIntroduction(t.Context(), publisherJob, prepared.operation)
			if err != nil || accepted == nil {
				t.Fatalf("registered Publisher after Source retirement: %v", err)
			}
			publisher.mu.Lock()
			unchanged := publisher.prefix == nil && publisher.permission == permission && permission.reserved == reserved && publisher.responder.prefix == nil
			publisher.mu.Unlock()
			if !unchanged {
				t.Fatal("pre-dial acceptance created network work or changed allocation")
			}
			for cycle := range 2 {
				if err := publisher.prepareTextResponder(t.Context(), publisherJob, accepted); err != nil {
					t.Fatalf("responder cycle %d: %v", cycle, err)
				}
				publisher.mu.Lock()
				sourcePrefix, dataPrefix := publisher.prefix, publisher.responder.prefix
				same := publisher.permission == permission && permission.batches == 2
				publisher.mu.Unlock()
				if sourcePrefix == nil || dataPrefix == nil || !same {
					t.Fatal("responder lost retained permission or repeated bootstrap")
				}
				if err := sourcePrefix.Close(); err != nil {
					t.Fatal(err)
				}
				if err := dataPrefix.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
