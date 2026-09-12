//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func liveTextCapsuleJob(t *testing.T, owner *textContext) *textJobIdentity {
	t.Helper()
	job, err := beginTextTestJob(t, owner, owner.endpoint, owner.surface)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := broker.New(broker.Config{ID: job.nonce, Grants: []broker.Grant{{Principal: fixtureID(218), Surface: broker.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	job.bound, job.workerGrant, owner.verifiedJob = true, grant, job
	owner.mu.Unlock()
	return job
}

// Real15Node registration/publication/lookup/issuance/delivery/Responder plus real Instance HPKE
// and both Service owners. Accepted State/worker qualification and the data
// joining transport are explicit fixtures: no network JOIN claim.
func TestTextIntroductionCapsuleBindsRealInstanceAndServiceStream(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, publisher, source := startTextRoleNetwork(t, carrier, true, true)
			// Rendezvous eligibility is a public-State fixture. It is never dialed
			// here; the actual data-pair transport below is a separate explicit seam.
			source.mu.Lock()
			source.view.NodeCount, source.snapshot.CandidateCount = 16, 16
			source.view.Nodes[15] = state.ClosedRouteNodeView{NodeID: fixtureID(202), RecordDigest: fixtureID(203), DutyGeneration: 16, RoleDomain: 2, Subrole: 4}
			candidate := source.snapshot.Candidates[4]
			candidate.NodeID, candidate.RecordDigest, candidate.FamilyID = fixtureID(202), fixtureID(203), fixtureID(204)
			candidate.PublicKey = fixtureID(205)
			source.snapshot.Candidates[15] = candidate
			source.mu.Unlock()
			public, authority, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(authority)
			now := time.Now().UTC().Truncate(time.Second)
			root, binding := acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), endpoint.network, authority, now.Add(-time.Second), source.view.Profile.NotAfter)
			t.Cleanup(func() {
				if err := endpoint.Close(); err != nil {
					t.Error(err)
				}
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			publications, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publications, [32]byte(public)
			if _, err := publisher.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := publisher.openTextIntroductionPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			registered, err := publisher.registerTextIntroduction(t.Context(), 1, time.Now().UTC().Add(120*time.Second).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			descriptor, err := publisher.publishTextDescriptor(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			reader := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			source.issuePermission(t, reader, [3]uint32{64, 64, 0})
			readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
			now = time.Now().UTC()
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			attempt, err := reader.prepareTextIntroduction(t.Context(), readerJob, targetlink.Link{Network: endpoint.network, Target: descriptor.Descriptor.Target}, bounds)
			if err != nil {
				t.Fatal(err)
			}
			_, sealed, err := route.DecodeClosedIntroductionSubmission(attempt.operation)
			if err != nil || len(attempt.operation) != 4096 {
				t.Fatalf("capsule operation: %v", err)
			}
			for _, hidden := range [][32]byte{attempt.plaintext.Target, attempt.plaintext.RendezvousNode, attempt.plaintext.JoinSecret, attempt.plaintext.ConnectionNonce, readerJob.nonce} {
				if bytes.Contains(attempt.operation, hidden[:]) {
					t.Fatal("recipient-only/local binding exposed outside HPKE")
				}
			}
			mutations := []struct {
				name   string
				change func(*route.ClosedIntroductionPlaintext)
			}{
				{"Target", func(value *route.ClosedIntroductionPlaintext) { value.Target = fixtureID(190) }},
				{"Rendezvous", func(value *route.ClosedIntroductionPlaintext) { value.RendezvousNode = source.view.Nodes[4].NodeID }},
				{"authority bounds", func(value *route.ClosedIntroductionPlaintext) {
					value.WorkSafetyMaximum = descriptor.Current.Credential.NotAfter + 1
				}},
			}
			for index, mutation := range mutations {
				altered := attempt.plaintext
				mutation.change(&altered)
				envelope := route.ClosedIntroductionCapsule{Slot: sealed.Slot, Revision: sealed.Revision, Expiry: sealed.Expiry, DeliveryNonce: fixtureID(byte(180 + index))}
				changed, _, err := route.SealClosedIntroduction(envelope, descriptor.Descriptor.Private.RecipientKey, altered)
				if err != nil {
					t.Fatal(err)
				}
				operation, err := route.EncodeClosedIntroductionSubmission(fixtureID(byte(170+index)), changed)
				if err != nil {
					t.Fatal(err)
				}
				if accepted, err := publisher.acceptTextIntroduction(t.Context(), publisherJob, operation); err == nil || accepted != nil || strings.Contains(err.Error(), "rate unavailable") {
					t.Fatalf("%s was not rejected by authority validation: %v", mutation.name, err)
				}
			}
			publisher.mu.Lock()
			noResponder := publisher.responder.prefix == nil && publisher.responder.set == nil
			beforeForward := publisher.permission.reserved[1]
			publisher.mu.Unlock()
			if !noResponder {
				t.Fatal("refused capsule created Responder work")
			}
			type deliveryOutcome struct {
				attempt *textIntroductionAttempt
				err     error
			}
			received := make(chan deliveryOutcome, 1)
			go func() {
				accepted, err := publisher.receiveTextIntroduction(t.Context(), publisherJob)
				received <- deliveryOutcome{accepted, err}
			}()
			if err := reader.submitTextIntroduction(t.Context(), readerJob, attempt); err != nil {
				_ = registered.channel.Close()
				<-received
				t.Fatal(err)
			}
			result := <-received
			accepted, err := result.attempt, result.err
			if err != nil {
				t.Fatal(err)
			}
			publisher.mu.Lock()
			responder := publisher.responder.prefix
			distinct := responder != nil && responder != publisher.prefix && responder != publisher.introduction.prefix &&
				publisher.responder.set != nil && publisher.responder.set != publisher.sourceSet && publisher.responder.set != publisher.introduction.set &&
				publisher.responder.set.interior[0].Domain == 3 && publisher.permission.reserved[1] > beforeForward && publisher.responder.opening == nil
			publisher.mu.Unlock()
			if !distinct {
				t.Fatal("accepted delivery did not establish independently issued Responder forwarding")
			}
			if err := publisher.prepareTextResponder(t.Context(), publisherJob, accepted); err != nil {
				t.Fatal(err)
			}
			publisher.mu.Lock()
			reused := publisher.responder.prefix == responder
			publisher.mu.Unlock()
			if !reused {
				t.Fatal("second accepted attempt replaced live Responder prefix")
			}
			checkTextResponderRetirementBoundary(t, publisher, publisherJob, accepted, source)
			responder = publisher.responder.prefix
			if accepted.digest != attempt.digest || accepted.binding.logical != attempt.binding.logical || accepted.plaintext != attempt.plaintext {
				t.Fatal("recipient changed the authenticated Attachment or logical context")
			}
			if _, err := publisher.acceptTextIntroduction(t.Context(), publisherJob, attempt.operation); err == nil || !strings.Contains(err.Error(), "replay") {
				t.Fatalf("replayed capsule accepted: %v", err)
			}
			if publisher.introductionReplays[sealed.DeliveryNonce] != registered.request.Expiry.Add(60*time.Second) {
				t.Fatal("replay retention does not cover original signed slot expiry")
			}
			exchangeTextCapsuleService(t, attempt, accepted)
			checkTextCapsuleAdmissionBoundaries(t, publisher, reader, source, publisherJob, attempt, descriptor.Descriptor.Private.RecipientKey)
			checkTextPreparationCallerHandover(t, reader, readerJob, source, targetlink.Link{Network: endpoint.network, Target: descriptor.Descriptor.Target}, bounds)
			publisher.retireJob(publisherJob)
			if err := publisher.finishJobCleanup(publisherJob, nil); err != nil {
				t.Fatal(err)
			}
			select {
			case <-responder.Done():
				t.Fatal("worker loss retired surviving context's Responder prefix")
			default:
			}
			if err := publisher.prepareTextResponder(t.Context(), publisherJob, accepted); err == nil {
				t.Fatal("retired Publisher job reused Responder authority")
			}
			if err := publisher.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-responder.Done():
			default:
				t.Fatal("context Close did not join Responder prefix")
			}
			if err := registered.recipient.Close(); err != nil {
				t.Fatal(err)
			}
			if _, _, err := route.OpenClosedIntroduction(sealed, source.view.Profile.Digest, registered.recipient, time.Now()); err == nil {
				t.Fatal("retired Instance recipient decrypted capsule")
			}
		})
	}
}

func exchangeTextCapsuleService(t *testing.T, reader, publisher *textIntroductionAttempt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	client, server := net.Pipe()
	snapshot, err := textdocument.NewSnapshot([]byte("recipient-only capsule bound this Service"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		stream, err := publisher.binding.openTextServiceStream(ctx, server, publisher.digest)
		if err != nil {
			done <- err
			return
		}
		err = snapshot.Respond(stream, stream)
		if err == nil {
			err = stream.CloseInput()
		}
		if err == nil {
			result := <-stream.Done()
			if result.Class != applicationconnection.CleanClose {
				err = errors.New("capsule Publisher terminal failed")
			}
		}
		done <- errors.Join(err, stream.Close())
	}()
	stream, err := reader.binding.openTextServiceStream(ctx, client, reader.digest)
	if err != nil {
		cancel()
		_ = client.Close()
		<-done
		t.Fatal(err)
	}
	body, err := textdocument.Read(ctx, stream)
	closeErr := stream.Close()
	publisherErr := <-done
	if err != nil || closeErr != nil || publisherErr != nil || string(body) != "recipient-only capsule bound this Service" {
		t.Fatalf("capsule-bound document: %q %v %v %v", body, err, closeErr, publisherErr)
	}
}

func TestTextIntroductionReplayAndOpeningRateAreContextBounded(t *testing.T) {
	owner := &textContext{}
	now := time.Now().UTC()
	for index := 0; index < 4; index++ {
		if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(byte(index+1)), now); err != nil {
			t.Fatal(err)
		}
	}
	if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(8), now.Add(time.Second-time.Nanosecond)); err == nil {
		t.Fatal("more than four openings in one second")
	}
	if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(8), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(9), now); err == nil {
		t.Fatal("clock rollback reopened rate allowance")
	}
	owner.introductionReplays = map[[32]byte]time.Time{fixtureID(10): now.Add(2 * time.Second)}
	if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(10), now.Add(time.Second)); err == nil {
		t.Fatal("replay allowed before expiry")
	}
	if err := owner.reserveTextIntroductionOpeningLocked(fixtureID(10), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
}
