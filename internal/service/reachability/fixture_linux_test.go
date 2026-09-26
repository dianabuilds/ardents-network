//go:build linux

package reachability_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// ADR-0105 retired the generation-2 Descriptor writer (Issue) and the
// generation-2 Store writers/readers (Publish, Lookup); only the private v3
// path composes Descriptors. These fixtures therefore exist solely for the
// Linux private-path suites, and the retained decoder and floor comparison
// remain governed by F-32.

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
	credential, err := (publication.Credential{InstancePublic: instance, Generation: 1,
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

type storeFixture struct {
	now          time.Time
	network      [32]byte
	authority    ed25519.PublicKey
	authorityKey ed25519.PrivateKey
	instance     ed25519.PrivateKey
	credential   publication.Credential
	current      publication.Current
}

func newStoreFixture(t *testing.T) storeFixture {
	t.Helper()
	now := time.Unix(2_000_100_000, 0).UTC()
	network := [32]byte{31}
	authority, authorityKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instance, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var fixedInstance [32]byte
	copy(fixedInstance[:], instancePublic)
	credential, err := (publication.Credential{InstancePublic: fixedInstance, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Minute).Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture := storeFixture{now: now, network: network, authority: authority, authorityKey: authorityKey, instance: instance, credential: credential}
	fixture.current = fixture.publish(t, "first-acknowledgement")
	return fixture
}

func (fixture storeFixture) publish(t *testing.T, acknowledgement string) publication.Current {
	t.Helper()
	owner, err := publication.Open(publication.Config{Root: t.TempDir(), NetworkID: fixture.network, Authority: fixture.authority, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	current, err := owner.Publish(context.Background(), publication.PublishInput{Credential: fixture.credential, InstanceSigner: fixture.instance,
		Acknowledgement: []byte(acknowledgement), At: fixture.now})
	if err != nil {
		t.Fatal(err)
	}
	return current
}
