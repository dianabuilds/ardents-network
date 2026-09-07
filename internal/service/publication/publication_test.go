package publication

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublishAcquireDrainAndUnpublish(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	current, err := owner.Publish(context.Background(), fixture.input(t, 1))
	if err != nil || current.Credential.Generation != 1 || len(current.Record) != publicationSize {
		t.Fatalf("Publish = %+v, %v", current, err)
	}
	if string(current.Record) == string(fixture.private) {
		t.Fatal("public publication retained its Instance private key")
	}
	decoded, decodeErr := Decode(current.Record, fixture.authority, fixture.network, fixture.now)
	if decodeErr != nil || decoded.Credential != current.Credential || decoded.Digest != current.Digest {
		t.Fatalf("Decode publication = %+v, %v", decoded, decodeErr)
	}
	lease, err := owner.Acquire(context.Background())
	if err != nil || lease.Current().Digest != current.Digest {
		t.Fatalf("Acquire = %+v, %v", lease, err)
	}
	if signature, signErr := lease.Sign(rand.Reader, []byte("lease-proof"), crypto.Hash(0)); signErr != nil ||
		!ed25519.Verify(fixture.public, []byte("lease-proof"), signature) {
		t.Fatalf("lease did not retain exact Instance signer: %v", signErr)
	}
	drained := make(chan error, 1)
	go func() { drained <- owner.Unpublish(context.Background()) }()
	select {
	case err := <-drained:
		t.Fatalf("Unpublish returned before retained acquisition drained: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if _, err := owner.Acquire(context.Background()); err == nil {
		t.Fatal("withdrawn publication remained acquirable")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-drained; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(owner.root.path, "generations", publicationGeneration(1))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("withdrawn generation remained on disk: %v", err)
	}
}

func TestPublishRetainsNonExportingInstanceSigner(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	input := fixture.input(t, 1)
	input.InstanceSigner = &nonExportingTestSigner{private: fixture.private}
	if _, err := owner.Publish(t.Context(), input); err != nil {
		t.Fatalf("publish with non-exporting signer: %v", err)
	}
	lease, err := owner.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	message := []byte("non-exporting publication signer")
	signature, err := lease.Sign(nil, message, crypto.Hash(0))
	if err != nil || !ed25519.Verify(fixture.public, message, signature) {
		t.Fatalf("retained non-exporting signer: %v", err)
	}
}

func TestPersistedPublicationIsNotLiveAfterRestartAndFloorSurvives(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Publish(context.Background(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	// Model a process loss: the public record remains, while the volatile
	// Instance key and in-memory current publication disappear.
	if err := owner.root.lease.release(); err != nil {
		t.Fatal(err)
	}
	owner.root.closed = true
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Acquire(context.Background()); err == nil {
		t.Fatal("durable public record was incorrectly revived as a live Instance")
	}
	if _, err := reopened.Publish(context.Background(), fixture.input(t, 1)); err == nil {
		t.Fatal("restart forgot persisted publication floor")
	}
	current, err := reopened.Publish(context.Background(), fixture.input(t, 2))
	if err != nil || current.Credential.Generation != 2 {
		t.Fatalf("higher live publication after restart = %+v, %v", current, err)
	}
}

func TestOpenRejectsSurplusOrTamperedPublicationState(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Publish(context.Background(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	if err := owner.root.lease.release(); err != nil {
		t.Fatal(err)
	}
	owner.root.closed = true
	if err := os.Mkdir(filepath.Join(root, "generations", publicationGeneration(2)), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(fixture.config(root)); err == nil {
		t.Fatal("surplus immutable generation was accepted")
	}
}

func TestCloseWithdrawsBeforeWaitingAndRejectsConcurrentPublish(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Publish(context.Background(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := owner.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned before retained acquisition drained: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if _, err := owner.Acquire(context.Background()); err == nil {
		t.Fatal("closing publication remained acquirable")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Publish(context.Background(), fixture.input(t, 2)); err == nil {
		t.Fatal("closed publication accepted a new generation")
	}
}

func TestCanceledUnpublishRetainsCleanupForSuccessorAndClose(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := owner.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	unpublished := make(chan error, 1)
	go func() { unpublished <- owner.Unpublish(ctx) }()
	select {
	case <-lease.generation.withdrawnAt:
	case <-t.Context().Done():
		t.Fatal("Unpublish did not withdraw the retained generation")
	}
	cancel()
	if err := <-unpublished; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Unpublish = %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 2)); err != nil {
		t.Fatalf("successor Publish after canceled drain = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "generations"))
	if err != nil || len(entries) != 1 || entries[0].Name() != publicationGeneration(2) {
		t.Fatalf("successor generations = %+v, %v", entries, err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("Close after canceled drain left root unreopenable: %v", err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledSupersedingPublishRetainsCleanupOwnership(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := owner.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	published := make(chan error, 1)
	go func() {
		_, publishErr := owner.Publish(ctx, fixture.input(t, 2))
		published <- publishErr
	}()
	select {
	case <-lease.generation.withdrawnAt:
	case <-t.Context().Done():
		t.Fatal("superseding Publish did not withdraw the retained generation")
	}
	cancel()
	if err := <-published; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled superseding Publish = %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 2)); err != nil {
		t.Fatalf("retry Publish after canceled drain = %v", err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("retry Publish left root unreopenable: %v", err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseRetainsRootLeaseUntilCanceledDrainFinishes(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := owner.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	unpublished := make(chan error, 1)
	go func() { unpublished <- owner.Unpublish(ctx) }()
	select {
	case <-lease.generation.withdrawnAt:
	case <-t.Context().Done():
		t.Fatal("Unpublish did not withdraw before cancellation")
	}
	cancel()
	if err := <-unpublished; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Unpublish = %v", err)
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	closeJoined := false
	t.Cleanup(func() {
		_ = lease.Close()
		if closeJoined {
			return
		}
		select {
		case closeErr := <-closed:
			if closeErr != nil {
				t.Errorf("Close cleanup = %v", closeErr)
			}
		case <-time.After(time.Second):
			t.Error("Close cleanup did not join")
		}
	})
	select {
	case <-owner.root.closeDrainStarted:
	case <-t.Context().Done():
		t.Fatal("Close did not enter its retained-drain boundary")
	}
	select {
	case closeErr := <-closed:
		closeJoined = true
		t.Fatalf("Close returned before retained Lease drained: %v", closeErr)
	default:
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	closeErr := <-closed
	closeJoined = true
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}

func TestRepeatedCloseLeavesSuccessorOwnerCurrent(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	first, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err = second.Publish(t.Context(), fixture.input(t, 2)); err != nil {
		t.Fatal(err)
	}
	if err = first.Close(); err != nil {
		t.Fatalf("repeated Close = %v", err)
	}
	pointer, exists, err := readPointer(root)
	if err != nil || !exists || pointer != publicationGeneration(2) {
		t.Fatalf("repeated Close changed successor pointer: %q, %t, %v", pointer, exists, err)
	}
	lease, err := second.Acquire(t.Context())
	if err != nil {
		t.Fatalf("repeated Close removed successor current: %v", err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUnpublishRetryRepairsPointerBeforeRetiredGeneration(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	owner.root.mu.Lock()
	current := owner.root.current
	withdraw(current)
	owner.root.current = nil
	owner.root.retiring = current
	owner.root.mu.Unlock()
	if err = owner.Unpublish(t.Context()); err == nil {
		t.Fatal("retry Unpublish accepted a root without a live generation")
	}
	if err = owner.root.lease.release(); err != nil {
		t.Fatal(err)
	}
	owner.root.mu.Lock()
	owner.root.closed = true
	owner.root.released = true
	owner.root.mu.Unlock()
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("retry Unpublish left a stale durable pointer: %v", err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialBindsSeparateIntroductionHPKEPublic(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	credential := fixture.input(t, 1).Credential
	var authority [32]byte
	copy(authority[:], fixture.authority)
	if err := Validate(credential, authority, fixture.network, fixture.now, connectCapability); err != nil {
		t.Fatalf("Validate Credential = %v", err)
	}

	tampered := credential
	tampered.IntroductionHPKEPublic[0] ^= 1
	if err := Validate(tampered, authority, fixture.network, fixture.now, connectCapability); err == nil {
		t.Fatal("Credential accepted an unsigned replacement Introduction HPKE key")
	}

	missing := credential
	missing.IntroductionHPKEPublic = [32]byte{}
	if _, err := missing.Issue(fixture.authPriv); err == nil {
		t.Fatal("Issue accepted a missing Introduction HPKE key")
	}

	encoded := encodeCredential(credential)
	encoded[1] = 1
	if _, err := decodeCredential(encoded); err == nil {
		t.Fatal("Decode accepted Credential v1")
	}
}

type publicationFixture struct {
	now              time.Time
	network          [32]byte
	introductionHPKE [32]byte
	authority        ed25519.PublicKey
	private          ed25519.PrivateKey
	public           ed25519.PublicKey
	authPriv         ed25519.PrivateKey
}

type nonExportingTestSigner struct{ private ed25519.PrivateKey }

func (signer *nonExportingTestSigner) Public() crypto.PublicKey { return signer.private.Public() }

func (signer *nonExportingTestSigner) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return signer.private.Sign(random, digest, opts)
}

func newPublicationFixture(t *testing.T) publicationFixture {
	t.Helper()
	authority, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return publicationFixture{now: time.Unix(2_000_000_000, 0), network: [32]byte{1}, introductionHPKE: [32]byte{9}, authority: authority,
		authPriv: authorityPrivate, private: private, public: public}
}

func (fixture publicationFixture) config(root string) Config {
	return Config{Root: root, NetworkID: fixture.network, Authority: fixture.authority, Clock: func() time.Time { return fixture.now }}
}

func (fixture publicationFixture) input(t *testing.T, generation uint64) PublishInput {
	t.Helper()
	var instance [32]byte
	copy(instance[:], fixture.public)
	credential, err := (Credential{InstancePublic: instance, IntroductionHPKEPublic: fixture.introductionHPKE, Generation: generation,
		NotBefore: fixture.now.Add(-time.Minute).Unix(), NotAfter: fixture.now.Add(time.Minute).Unix(),
		NetworkID: fixture.network, Capabilities: publishCapability | connectCapability}).Issue(fixture.authPriv)
	if err != nil {
		t.Fatal(err)
	}
	return PublishInput{Credential: credential, InstanceSigner: fixture.private, Acknowledgement: []byte("fresh-introduction-proof"), At: fixture.now}
}
