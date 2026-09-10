//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Accepted State, installed launch and Introduction readiness are explicit
// fixtures. Publication roots/signatures, both Endpoint Service owners, TLS,
// exporter binding, native authentication and the document exchange are real.
func textServiceFixture(t *testing.T) (*textServiceBinding, *textServiceBinding, publication.Current) {
	t.Helper()
	now := time.Now().UTC()
	profile := state.ClosedProfileView{NetworkID: fixtureID(1), StateGeneration: fixtureID(2), StateDigest: fixtureID(3),
		Digest: fixtureID(4), IssuanceAuthorityKey: fixtureID(5), IssuerNodeID: fixtureID(6), IssuerDutyGeneration: 1,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(2 * time.Hour)}
	newPeer := func(surface broker.Surface) (*textContext, *textJobIdentity) {
		peer, principal := textContextEndpoint(t)
		peer.network, peer.clock = profile.NetworkID, time.Now
		peer.closedState = &textPermissionStateFixture{profile: profile}
		t.Cleanup(func() {
			if err := peer.Close(); err != nil {
				t.Error(err)
			}
		})
		owner := admittedTextContext(t, peer, principal, surface)
		job, err := beginTextTestJob(t, owner, peer, surface)
		if err != nil {
			t.Fatal(err)
		}
		grant, err := broker.New(broker.Config{ID: job.nonce, Grants: []broker.Grant{{Principal: principal, Surface: broker.Connection}}})
		if err != nil {
			t.Fatal(err)
		}
		job.bound, job.workerGrant, owner.verifiedJob = true, grant, job
		return owner, job
	}
	reader, readerJob := newPeer(broker.Connection)
	publisher, publisherJob := newPeer(broker.Administration)
	authorityPublic, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(authority); clear(signer) })
	root, err := publication.Open(publication.Config{Root: publicationStoreRoot(t), NetworkID: profile.NetworkID, Authority: authorityPublic})
	if err != nil {
		t.Fatal(err)
	}
	publisher.endpoint.publications = root
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	credential := publication.Credential{NetworkID: profile.NetworkID, IntroductionHPKEPublic: fixtureID(7),
		Generation: 1, NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Hour).Unix(),
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}
	copy(credential.InstancePublic[:], instancePublic)
	credential, err = credential.Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	current, err := root.Publish(t.Context(), publication.PublishInput{Credential: credential, InstanceSigner: signer,
		Acknowledgement: []byte("explicit isolated Introduction readiness fixture"), At: now})
	if err != nil {
		t.Fatal(err)
	}
	bounds := [3]int64{now.Add(30 * time.Second).Unix(), now.Add(30 * time.Second).Unix(), now.Add(30 * time.Second).Unix()}
	clientBinding, err := reader.newTextServiceBinding(readerJob, targetlink.Link{Network: profile.NetworkID, Target: current.Credential.Target}, current, bounds)
	if err != nil {
		t.Fatal(err)
	}
	publisherBinding, err := publisher.acceptTextServiceBinding(publisherJob, current, clientBinding.facts)
	if err != nil {
		t.Fatal(err)
	}
	if clientBinding.logical != publisherBinding.logical {
		t.Fatal("independent Endpoint contexts disagree")
	}
	return clientBinding, publisherBinding, current
}

func TestTextServiceRealTLSAndDocumentExchange(t *testing.T) {
	for _, size := range []int{0, 64 << 10, textdocument.MaximumBytes} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			clientBinding, publisherBinding, _ := textServiceFixture(t)
			clientRoute, publisherRoute := net.Pipe()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			body := bytes.Repeat([]byte("t"), size)
			document, err := textdocument.NewSnapshot(body)
			if err != nil {
				t.Fatal(err)
			}
			publisherDone := make(chan error, 1)
			go func() {
				stream, err := publisherBinding.openTextServiceStream(ctx, publisherRoute, fixtureID(51))
				if err != nil {
					publisherDone <- err
					return
				}
				err = document.Respond(stream, stream)
				if err == nil {
					err = stream.CloseInput()
				}
				if err == nil {
					outcome := <-stream.Done()
					if outcome.Class != applicationconnection.CleanClose {
						err = errors.New("Publisher stream did not complete")
					}
				}
				publisherDone <- errors.Join(err, stream.Close())
			}()
			stream, err := clientBinding.openTextServiceStream(ctx, clientRoute, fixtureID(51))
			if err != nil {
				cancel()
				t.Fatalf("client setup: %v; Publisher: %v", err, <-publisherDone)
			}
			received, readErr := textdocument.Read(ctx, stream)
			publisherErr := <-publisherDone
			if readErr != nil || publisherErr != nil || !bytes.Equal(received, body) {
				t.Fatalf("document=%d expected=%d client=%v Publisher=%v", len(received), len(body), readErr, publisherErr)
			}
		})
	}
}

func TestTextServiceRejectsForeignTupleAndExpiredLocalJob(t *testing.T) {
	client, publisher, current := textServiceFixture(t)
	for _, changed := range []string{"network", "target", "instance", "publication", "profile", "generation", "zero-nonce", "zero-binding", "expired", "extended"} {
		t.Run(changed, func(t *testing.T) {
			facts := client.facts
			switch changed {
			case "network":
				facts.Network[0]++
			case "target":
				facts.Target[0]++
			case "instance":
				facts.InstancePublic[0]++
			case "publication":
				facts.PublicationDigest[0]++
			case "profile":
				facts.ProfileDigest[0]++
			case "generation":
				facts.InstanceGeneration++
			case "zero-nonce":
				facts.ConnectionNonce = [32]byte{}
			case "zero-binding":
				facts.InitiatorBinding = [32]byte{}
			case "expired":
				facts.NoNewRecoveryAfter = time.Now().Add(-time.Second).Unix()
			case "extended":
				facts.WorkSafetyMaximum = current.Credential.NotAfter + 1
			}
			if accepted, err := publisher.owner.acceptTextServiceBinding(publisher.job, current, facts); err == nil || accepted != nil {
				t.Fatal("foreign or incompatible capsule tuple accepted")
			}
		})
	}
	client.owner.retireJob(client.job)
	if err := client.current(); err == nil {
		t.Fatal("retired reader context accepted")
	}
	if _, err := client.owner.newTextServiceBinding(client.job, targetlink.Link{Network: current.Credential.NetworkID, Target: current.Credential.Target},
		current, [3]int64{client.facts.WorkSafetyNotAfter, client.facts.WorkSafetyMaximum, client.facts.NoNewRecoveryAfter}); err == nil {
		t.Fatal("retired worker created another Service binding")
	}
}

func TestTextServiceFreshCommitmentDoesNotExposeLocalJob(t *testing.T) {
	client, _, current := textServiceFixture(t)
	other, err := client.owner.newTextServiceBinding(client.job,
		targetlink.Link{Network: current.Credential.NetworkID, Target: current.Credential.Target}, current,
		[3]int64{client.facts.WorkSafetyNotAfter, client.facts.WorkSafetyMaximum, client.facts.NoNewRecoveryAfter})
	if err != nil {
		t.Fatal(err)
	}
	if other.facts.InitiatorBinding == client.facts.InitiatorBinding || other.facts.ConnectionNonce == client.facts.ConnectionNonce ||
		other.logical == client.logical || client.facts.InitiatorBinding == client.job.nonce {
		t.Fatal("reused or exposed a private job binding")
	}
	corrupt := current
	corrupt.Record = bytes.Clone(current.Record)
	corrupt.Record[len(corrupt.Record)-1]++
	if _, err := client.owner.newTextServiceBinding(client.job,
		targetlink.Link{Network: current.Credential.NetworkID, Target: current.Credential.Target}, corrupt,
		[3]int64{client.facts.WorkSafetyNotAfter, client.facts.WorkSafetyMaximum, client.facts.NoNewRecoveryAfter}); err == nil {
		t.Fatal("corrupt publication accepted")
	}
}

func TestTextServiceDifferentCapsuleCannotAuthenticateSameTLS(t *testing.T) {
	client, publisher, _ := textServiceFixture(t)
	left, right := net.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		stream, err := publisher.openTextServiceStream(ctx, right, fixtureID(61))
		if stream != nil {
			_ = stream.Close()
			err = errors.New("foreign Attachment exposed Publisher stream")
		}
		finished <- err
	}()
	stream, err := client.openTextServiceStream(ctx, left, fixtureID(62))
	if err == nil || stream != nil {
		t.Fatal("different capsule yielded an authenticated stream")
	}
	if err := <-finished; err == nil {
		t.Fatal("Publisher accepted different Attachment")
	}
}

func TestTextServiceProtectedContextSeparatesAttachmentFromLogicalIdentity(t *testing.T) {
	client, _, _ := textServiceFixture(t)
	first, err := nativeconnection.ProtectedAttachmentContext(client.logical, fixtureID(70), 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := nativeconnection.ProtectedAttachmentContext(client.logical, fixtureID(71), 2)
	if err != nil {
		t.Fatal(err)
	}
	logical, err := nativeconnection.ProtectedContext(client.facts)
	if err != nil || logical != client.logical || first == second || first == logical || second == logical {
		t.Fatal("Attachment replacement changed or collapsed logical context")
	}
}

// Component-only binding fixture; actual Publisher admission verifies its capsule.
// acceptTextServiceBinding verifies the shared capsule tuple against the
// Publisher's own publication and authorization. It does not silently clamp
// incompatible bounds or treat a requester's nonce as authority.
func (owner *textContext) acceptTextServiceBinding(job *textJobIdentity, current publication.Current,
	facts nativeconnection.ProtectedContextInput) (*textServiceBinding, error) {
	if owner == nil {
		return nil, errors.New("text Service context unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveTextServiceJobLocked(job, broker.Administration) {
		return nil, errors.New("text Service Publisher job unavailable")
	}
	return owner.bindTextServiceLocked(job, current, facts)
}
