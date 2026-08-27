package reachability_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestDescriptorBindsPublicationTargetAndLiveIntroduction(t *testing.T) {
	t.Parallel()
	fixture := newDescriptorFixture(t)
	raw, issued, err := reachability.Issue(reachability.IssueInput{Current: fixture.current, InstanceSigner: fixture.instancePrivate,
		Introduction: fixture.introduction()})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := reachability.Verify(raw, fixture.current.Credential.Target, fixture.network, fixture.now)
	if err != nil || verified.Current.Digest != fixture.current.Digest ||
		verified.Descriptor.Introduction.StateDigest != issued.Introduction.StateDigest ||
		verified.Descriptor.Introduction.Epoch != issued.Introduction.Epoch ||
		verified.Descriptor.Introduction.IntroductionNodeID != issued.Introduction.IntroductionNodeID ||
		verified.Descriptor.Introduction.RendezvousNodeID != issued.Introduction.RendezvousNodeID ||
		verified.Descriptor.Introduction.Reachability != issued.Introduction.Reachability ||
		verified.Descriptor.Introduction.JoinHandle != issued.Introduction.JoinHandle ||
		verified.Descriptor.Version != 1 || verified.Descriptor.Introduction.SubmissionMode != reachability.SubmissionFixedGrant ||
		!verified.Descriptor.Introduction.NotAfter.Equal(issued.Introduction.NotAfter) ||
		!bytes.Equal(verified.Descriptor.Introduction.SubmissionAuthorization, issued.Introduction.SubmissionAuthorization) ||
		string(verified.Descriptor.Publication) != string(fixture.current.Record) {
		t.Fatalf("Verify = %+v, %v", verified, err)
	}

	if _, err := reachability.Verify(raw, [32]byte{99}, fixture.network, fixture.now); err == nil {
		t.Fatal("Descriptor accepted a substituted Target")
	}
	mutated := append([]byte(nil), raw...)
	mutated[len(mutated)-1] ^= 1
	if _, err := reachability.Verify(mutated, fixture.current.Credential.Target, fixture.network, fixture.now); err == nil {
		t.Fatal("Descriptor accepted a changed Instance signature")
	}
	if _, err := reachability.Verify(raw, fixture.current.Credential.Target, fixture.network, fixture.now.Add(31*time.Second)); err == nil {
		t.Fatal("Descriptor accepted an expired live Introduction slot")
	}
}

func TestDescriptorV2DeclaresMembershipSubmissionWithoutServiceTicket(t *testing.T) {
	t.Parallel()
	fixture := newDescriptorFixture(t)
	slot := fixture.introduction()
	slot.SubmissionAuthorization = nil
	slot.SubmissionMode = reachability.SubmissionMembershipGrant
	raw, issued, err := reachability.Issue(reachability.IssueInput{Current: fixture.current, InstanceSigner: fixture.instancePrivate, Introduction: slot})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := reachability.Verify(raw, fixture.current.Credential.Target, fixture.network, fixture.now)
	if err != nil || issued.Version != 2 || verified.Descriptor.Version != 2 ||
		verified.Descriptor.Introduction.SubmissionMode != reachability.SubmissionMembershipGrant ||
		len(verified.Descriptor.Introduction.SubmissionAuthorization) != 0 {
		t.Fatalf("membership Descriptor = %+v, issued version %d, %v", verified.Descriptor.Introduction, issued.Version, err)
	}
	slot.SubmissionAuthorization = []byte("publisher-specific-ticket")
	if _, _, err := reachability.Issue(reachability.IssueInput{Current: fixture.current, InstanceSigner: fixture.instancePrivate, Introduction: slot}); err == nil {
		t.Fatal("Descriptor v2 accepted a publisher-specific membership ticket")
	}
}

func TestDescriptorRejectsSlotBeyondCredentialLifetime(t *testing.T) {
	t.Parallel()
	fixture := newDescriptorFixture(t)
	slot := fixture.introduction()
	slot.NotAfter = time.Unix(fixture.current.Credential.NotAfter+1, 0).UTC()
	if _, _, err := reachability.Issue(reachability.IssueInput{Current: fixture.current, InstanceSigner: fixture.instancePrivate, Introduction: slot}); err == nil {
		t.Fatal("Issue accepted a live Introduction slot after Credential expiry")
	}
}

type descriptorFixture struct {
	now             time.Time
	network         [32]byte
	current         publication.Current
	instancePrivate ed25519.PrivateKey
}

func newDescriptorFixture(t *testing.T) descriptorFixture {
	t.Helper()
	now := time.Unix(2_000_000_000, 0).UTC()
	network := [32]byte{1}
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (publication.Credential{InstancePublic: instance, IntroductionHPKEPublic: [32]byte{4}, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Minute).Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := publication.Open(publication.Config{Root: t.TempDir(), NetworkID: network, Authority: authorityPublic, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	current, err := owner.Publish(context.Background(), publication.PublishInput{Credential: credential, InstanceSigner: instancePrivate,
		Acknowledgement: []byte("introduction-ready"), At: now})
	if err != nil {
		t.Fatal(err)
	}
	return descriptorFixture{now: now, network: network, current: current, instancePrivate: instancePrivate}
}

func (fixture descriptorFixture) introduction() reachability.Introduction {
	return reachability.Introduction{StateDigest: [32]byte{2}, Epoch: 7, IntroductionNodeID: [32]byte{3}, RendezvousNodeID: [32]byte{4},
		Reachability: [32]byte{5}, JoinHandle: [32]byte{6}, NotAfter: fixture.now.Add(30 * time.Second), SubmissionAuthorization: []byte("slot-submit")}
}
