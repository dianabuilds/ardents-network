package publication

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	publicationPrefix           = "ardents-service-publication-v1\x00"
	publicationSize             = len(publicationPrefix) + credentialSize + 32 + ed25519.SignatureSize
	maximumAcknowledgementBytes = 4096
)

type volatileSigner struct {
	mu      sync.Mutex
	private ed25519.PrivateKey
}

func (signer *volatileSigner) Public() crypto.PublicKey {
	signer.mu.Lock()
	defer signer.mu.Unlock()
	if len(signer.private) != ed25519.PrivateKeySize {
		return nil
	}
	return signer.private.Public()
}

func (signer *volatileSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	signer.mu.Lock()
	defer signer.mu.Unlock()
	if len(signer.private) != ed25519.PrivateKeySize {
		return nil, errors.New("publication Instance signer was withdrawn")
	}
	return signer.private.Sign(rand, digest, opts)
}

func (signer *volatileSigner) erase() {
	signer.mu.Lock()
	defer signer.mu.Unlock()
	for index := range signer.private {
		signer.private[index] = 0
	}
	signer.private = nil
}

// Open acquires the exclusive C1 publication root and restores its floor. A
// persisted public record is intentionally not considered live after restart.
func Open(config Config) (*Publication, error) {
	if config.NetworkID == [32]byte{} || len(config.Authority) != ed25519.PublicKeySize {
		return nil, errors.New("publication configuration is incomplete")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	root, err := openDurableRoot(config)
	if err != nil {
		return nil, err
	}
	return &Publication{config: config, root: root}, nil
}

// Publish withdraws any former live generation, drains it, and then atomically
// exposes one higher immutable public generation. The former root writer is
// never consulted or modified.
func (publication *Publication) Publish(ctx context.Context, input PublishInput) (Current, error) {
	if publication == nil || publication.root == nil || ctx == nil {
		return Current{}, errors.New("publication is not open")
	}
	if err := ctx.Err(); err != nil {
		return Current{}, err
	}
	publication.opMu.Lock()
	defer publication.opMu.Unlock()
	if input.At.IsZero() {
		input.At = publication.config.Clock()
	}
	if len(input.Acknowledgement) == 0 || len(input.Acknowledgement) > maximumAcknowledgementBytes || input.InstanceSigner == nil {
		return Current{}, errors.New("publication input is incomplete")
	}
	var authority [32]byte
	copy(authority[:], publication.config.Authority)
	if err := validateCredential(input.Credential, authority, publication.config.NetworkID, input.At, publishCapability|connectCapability); err != nil {
		return Current{}, err
	}
	public, ok := input.InstanceSigner.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || string(public) != string(input.Credential.InstancePublic[:]) {
		return Current{}, errors.New("publication Instance signer does not match its Credential")
	}
	publication.root.mu.Lock()
	if publication.root.closed {
		publication.root.mu.Unlock()
		return Current{}, errors.New("publication is closed")
	}
	prior := publication.root.current
	if input.Credential.Generation <= publication.root.floor {
		publication.root.mu.Unlock()
		return Current{}, errors.New("publication generation is not higher than its floor")
	}
	if prior != nil {
		withdraw(prior)
		publication.root.current = nil
		if err := publication.root.removeCurrent(); err != nil {
			publication.root.mu.Unlock()
			return Current{}, err
		}
	}
	publication.root.mu.Unlock()
	if err := waitDrained(ctx, prior); err != nil {
		return Current{}, err
	}
	if prior != nil {
		prior.releaseSigner()
		if err := removeGeneration(publication.root.path, prior.credential.Generation); err != nil {
			return Current{}, err
		}
	}
	if err := publication.removePersistedUnavailable(); err != nil {
		return Current{}, err
	}
	record, digest, err := encodePublication(input.Credential, input.Acknowledgement, input.InstanceSigner)
	if err != nil {
		return Current{}, err
	}
	if err := writeFloor(publication.root.path, input.Credential.Generation); err != nil {
		return Current{}, fmt.Errorf("persist publication floor: %w", err)
	}
	if err := writeGeneration(publication.root.path, input.Credential.Generation, record); err != nil {
		return Current{}, err
	}
	if err := replacePointer(publication.root.path, publicationGeneration(input.Credential.Generation)); err != nil {
		return Current{}, fmt.Errorf("publish current publication: %w", err)
	}
	retained, release := retainInstanceSigner(input.InstanceSigner)
	current := &generation{credential: input.Credential, record: append([]byte(nil), record...), digest: digest,
		signer: retained, release: release, drained: make(chan struct{})}
	publication.root.mu.Lock()
	if publication.root.closed {
		publication.root.mu.Unlock()
		current.releaseSigner()
		return Current{}, errors.New("publication closed while publishing")
	}
	publication.root.floor, publication.root.current = input.Credential.Generation, current
	publication.root.mu.Unlock()
	return current.current(), nil
}

// Acquire retains one currently live generation. It never revives a durable
// record recovered without its volatile Instance private material.
func (publication *Publication) Acquire(ctx context.Context) (*Lease, error) {
	return publication.AcquireAt(ctx, time.Time{})
}

// Floor returns the durable highest committed publication generation.
func (publication *Publication) Floor() (uint64, error) {
	if publication == nil || publication.root == nil {
		return 0, errors.New("publication is not open")
	}
	publication.root.mu.Lock()
	defer publication.root.mu.Unlock()
	if publication.root.closed {
		return 0, errors.New("publication is closed")
	}
	return publication.root.floor, nil
}

// AcquireAt retains the current publication only when it is live at the
// caller's authenticated decision time. A zero time uses the owner clock.
func (publication *Publication) AcquireAt(ctx context.Context, at time.Time) (*Lease, error) {
	if publication == nil || publication.root == nil || ctx == nil {
		return nil, errors.New("publication is not open")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if at.IsZero() {
		at = publication.config.Clock()
	}
	publication.root.mu.Lock()
	defer publication.root.mu.Unlock()
	current := publication.root.current
	if publication.root.closed || current == nil || current.withdrawn || current.signer == nil ||
		at.Unix() < current.credential.NotBefore || at.Unix() >= current.credential.NotAfter {
		return nil, errors.New("no live publication is available")
	}
	current.refs++
	return &Lease{publication: publication, generation: current, current: current.current()}, nil
}

// Unpublish withdraws the current record before waiting for retained work; no
// subsequent acquisition can observe its private Instance signer.
func (publication *Publication) Unpublish(ctx context.Context) error {
	if publication == nil || publication.root == nil || ctx == nil {
		return errors.New("publication is not open")
	}
	publication.opMu.Lock()
	defer publication.opMu.Unlock()
	publication.root.mu.Lock()
	if publication.root.closed || publication.root.current == nil {
		publication.root.mu.Unlock()
		return errors.New("no live publication is available")
	}
	current := publication.root.current
	withdraw(current)
	publication.root.current = nil
	err := publication.root.removeCurrent()
	publication.root.mu.Unlock()
	if err != nil {
		return err
	}
	if err := waitDrained(ctx, current); err != nil {
		return err
	}
	current.releaseSigner()
	return removeGeneration(publication.root.path, current.credential.Generation)
}

// Close withdraws the live publication, drains retained users, erases private
// material, and finally releases the exclusive root lease.
func (publication *Publication) Close() error {
	if publication == nil || publication.root == nil {
		return nil
	}
	publication.opMu.Lock()
	defer publication.opMu.Unlock()
	publication.root.mu.Lock()
	if publication.root.closed {
		publication.root.mu.Unlock()
		return nil
	}
	publication.root.closed = true
	current := publication.root.current
	publication.root.current = nil
	if current != nil {
		withdraw(current)
	}
	err := publication.root.removeCurrent()
	publication.root.mu.Unlock()
	if err != nil {
		return err
	}
	if current != nil {
		<-current.drained
		current.releaseSigner()
		if err := removeGeneration(publication.root.path, current.credential.Generation); err != nil {
			return err
		}
	}
	return publication.root.lease.release()
}

func (publication *Publication) removePersistedUnavailable() error {
	publication.root.mu.Lock()
	defer publication.root.mu.Unlock()
	pointer, exists, err := readPointer(publication.root.path)
	if err != nil || !exists {
		return err
	}
	if err := publication.root.removeCurrent(); err != nil {
		return err
	}
	return os.RemoveAll(generationPath(publication.root.path, pointer))
}

func (generation *generation) current() Current {
	return Current{Credential: generation.credential, Digest: generation.digest, Record: append([]byte(nil), generation.record...)}
}

func withdraw(generation *generation) {
	if generation.withdrawn {
		return
	}
	generation.withdrawn = true
	if generation.refs == 0 {
		close(generation.drained)
	}
}

func waitDrained(ctx context.Context, generation *generation) error {
	if generation == nil {
		return nil
	}
	select {
	case <-generation.drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lease *Lease) Current() Current {
	if lease == nil {
		return Current{}
	}
	return Current{Credential: lease.current.Credential, Digest: lease.current.Digest, Record: append([]byte(nil), lease.current.Record...)}
}

func (lease *Lease) Public() crypto.PublicKey {
	if lease == nil || lease.generation == nil || lease.generation.signer == nil {
		return nil
	}
	return lease.generation.signer.Public()
}

func (lease *Lease) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if lease == nil || lease.generation == nil || lease.generation.signer == nil {
		return nil, errors.New("publication lease is closed")
	}
	return lease.generation.signer.Sign(rand, digest, opts)
}

func (lease *Lease) Close() error {
	if lease == nil || lease.publication == nil || lease.generation == nil {
		return nil
	}
	root := lease.publication.root
	root.mu.Lock()
	if lease.generation.refs == 0 {
		root.mu.Unlock()
		return nil
	}
	lease.generation.refs--
	if lease.generation.withdrawn && lease.generation.refs == 0 {
		close(lease.generation.drained)
	}
	lease.generation, lease.publication = nil, nil
	root.mu.Unlock()
	return nil
}

func retainInstanceSigner(signer crypto.Signer) (crypto.Signer, func()) {
	if private, ok := signer.(ed25519.PrivateKey); ok && len(private) == ed25519.PrivateKeySize {
		volatile := &volatileSigner{private: append(ed25519.PrivateKey(nil), private...)}
		return volatile, volatile.erase
	}
	return signer, func() {}
}

func publicationGeneration(value uint64) string { return fmt.Sprintf("%016x", value) }

func writeGeneration(root string, number uint64, record []byte) error {
	generations := filepath.Join(root, "generations")
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	if err := writeExclusive(filepath.Join(staging, "publication.bin"), record); err != nil {
		return err
	}
	final := generationPath(root, publicationGeneration(number))
	if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish immutable publication generation: %w", err)
	}
	return nil
}

func removeGeneration(root string, number uint64) error {
	if err := os.RemoveAll(generationPath(root, publicationGeneration(number))); err != nil {
		return fmt.Errorf("remove withdrawn publication generation: %w", err)
	}
	return nil
}

func encodePublication(credential Credential, acknowledgement []byte, signer crypto.Signer) ([]byte, [32]byte, error) {
	var digest [32]byte
	acknowledgementDigest := sha256.Sum256(acknowledgement)
	message := append([]byte(publicationPrefix), encodeCredential(credential)...)
	message = append(message, acknowledgementDigest[:]...)
	commitment := sha256.Sum256(message)
	signature, err := signer.Sign(nil, commitment[:], crypto.Hash(0))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, digest, errors.New("sign public publication: Instance signer failed")
	}
	record := append(message, signature...)
	digest = sha256.Sum256(record)
	return record, digest, nil
}

func loadGeneration(root, name string, config Config) (*generation, error) {
	record, err := readFile(filepath.Join(generationPath(root, name), "publication.bin"), int64(publicationSize))
	if err != nil || len(record) != publicationSize || string(record[:len(publicationPrefix)]) != publicationPrefix {
		return nil, errors.New("publication record is malformed")
	}
	credential, err := decodeCredential(record[len(publicationPrefix) : len(publicationPrefix)+credentialSize])
	var authority [32]byte
	copy(authority[:], config.Authority)
	if err != nil || validateCredential(credential, authority, config.NetworkID,
		time.Unix(credential.NotBefore, 0), connectCapability) != nil {
		return nil, errors.New("publication record Credential is invalid")
	}
	message := record[:publicationSize-ed25519.SignatureSize]
	commitment := sha256.Sum256(message)
	if !ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]), commitment[:], record[publicationSize-ed25519.SignatureSize:]) {
		return nil, errors.New("publication record Instance proof is invalid")
	}
	digest := sha256.Sum256(record)
	return &generation{credential: credential, record: record, digest: digest}, nil
}

// Decode verifies one immutable public publication record for a consumer. It
// cannot make that record live or expose Instance private material.
func Decode(record []byte, authority ed25519.PublicKey, network [32]byte, at time.Time) (Current, error) {
	if len(authority) != ed25519.PublicKeySize || network == [32]byte{} || at.IsZero() {
		return Current{}, errors.New("publication verification input is incomplete")
	}
	if len(record) != publicationSize || string(record[:len(publicationPrefix)]) != publicationPrefix {
		return Current{}, errors.New("publication record is malformed")
	}
	credential, err := decodeCredential(record[len(publicationPrefix) : len(publicationPrefix)+credentialSize])
	var authorityFixed [32]byte
	copy(authorityFixed[:], authority)
	if err != nil || validateCredential(credential, authorityFixed, network, at, connectCapability) != nil {
		return Current{}, errors.New("publication record Credential is invalid")
	}
	message := record[:publicationSize-ed25519.SignatureSize]
	commitment := sha256.Sum256(message)
	if !ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]), commitment[:], record[publicationSize-ed25519.SignatureSize:]) {
		return Current{}, errors.New("publication record Instance proof is invalid")
	}
	digest := sha256.Sum256(record)
	return Current{Credential: credential, Digest: digest, Record: append([]byte(nil), record...)}, nil
}
