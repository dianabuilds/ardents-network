package publication

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
)

// PublishAfterReadiness advances the durable generation floor before asking
// its owner for one bounded readiness transcript. It exposes the signed
// publication only after readiness succeeds. A readiness failure leaves the
// advanced floor in place and the generation unavailable.
func (publication *Publication) PublishAfterReadiness(ctx context.Context, input PublishInput,
	readiness func(context.Context) ([]byte, error),
) (Current, error) {
	if readiness == nil || len(input.Acknowledgement) != 0 {
		return Current{}, errors.New("publication readiness input is incomplete")
	}
	return publication.publish(ctx, input, readiness)
}

func (publication *Publication) publish(ctx context.Context, input PublishInput,
	readiness func(context.Context) ([]byte, error),
) (Current, error) {
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
	if readiness == nil && (len(input.Acknowledgement) == 0 || len(input.Acknowledgement) > maximumAcknowledgementBytes) {
		return Current{}, errors.New("publication input is incomplete")
	}
	if input.InstanceSigner == nil {
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

	var record []byte
	var digest [32]byte
	var err error
	if readiness == nil {
		record, digest, err = encodePublication(input.Credential, input.Acknowledgement, input.InstanceSigner)
		if err != nil {
			return Current{}, err
		}
	}
	if err := writeFloor(publication.root.path, input.Credential.Generation); err != nil {
		return Current{}, fmt.Errorf("persist publication floor: %w", err)
	}
	publication.root.mu.Lock()
	publication.root.floor = input.Credential.Generation
	publication.root.mu.Unlock()
	if readiness != nil {
		input.Acknowledgement, err = readiness(ctx)
		if err != nil {
			return Current{}, err
		}
		if len(input.Acknowledgement) == 0 || len(input.Acknowledgement) > maximumAcknowledgementBytes {
			return Current{}, errors.New("publication readiness transcript is invalid")
		}
		record, digest, err = encodePublication(input.Credential, input.Acknowledgement, input.InstanceSigner)
		if err != nil {
			return Current{}, err
		}
	}
	if err := writeGeneration(publication.root.path, input.Credential.Generation, record); err != nil {
		return Current{}, err
	}
	if err := replacePointer(publication.root.path, publicationGeneration(input.Credential.Generation)); err != nil {
		cleanupErr := removeGeneration(publication.root.path, input.Credential.Generation)
		return Current{}, errors.Join(fmt.Errorf("publish current publication: %w", err), cleanupErr)
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
	publication.root.current = current
	publication.root.mu.Unlock()
	return current.current(), nil
}
