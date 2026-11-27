package instance

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
)

// Publication signs only one exact genuine REGISTER receipt for this consumed
// Instance. It returns public evidence; Store ACK and accepting readiness are
// separate decisions of the live Publisher. A refresh retains these same bytes.
func (binding *Binding) Publication(ctx context.Context, registration *introduction.HolderRegistration) ([]byte, error) {
	if binding == nil || binding.root == nil || registration == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	receipt, err := registration.Receipt()
	if err != nil {
		return nil, err
	}
	if receipt.Facts().Network != binding.credential.Delegation().Network {
		return nil, ErrUnavailable
	}
	if err = binding.Consume(ctx); err != nil {
		return nil, err
	}
	root := binding.root
	root.mu.Lock()
	if err = binding.checkLocal(ctx); err != nil {
		root.mu.Unlock()
		return nil, err
	}
	binding.observations.Add(1)
	defer binding.observations.Done()
	if len(binding.record) == 0 {
		if len(binding.private) != ed25519.PrivateKeySize {
			root.mu.Unlock()
			return nil, ErrUnavailable
		}
		raw := publication.ServicePublicationTranscript(binding.credential, receipt.Facts().Acknowledgement)
		commitment := sha256.Sum256(raw)
		binding.record = append(raw, ed25519.Sign(binding.private, commitment[:])...)
		binding.receipt = receipt
	} else if binding.receipt != receipt {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	record := append([]byte(nil), binding.record...)
	root.mu.Unlock()
	// Never hold the private-material lock during genuine Network observation.
	if err = registration.CheckReceipt(receipt); err != nil {
		return nil, err
	}
	if err = binding.generation.Publish(ctx, record); err != nil {
		return nil, err
	}
	if err = registration.CheckReceipt(receipt); err != nil {
		return nil, err
	}
	err = binding.observe(ctx)
	if err != nil {
		return nil, errors.Join(ErrUnavailable, err)
	}
	return record, nil
}
