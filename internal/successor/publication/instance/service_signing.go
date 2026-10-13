package instance

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// This private signer is confined inside TLS and accepts only TLS1.3 server
// CertificateVerify input. No caller receives a crypto.Signer or raw key.
type serviceCertificateSigner struct{ service *ServiceAttachment }

func (signer *serviceCertificateSigner) Public() crypto.PublicKey {
	return append(ed25519.PublicKey(nil), signer.service.identity.Instance[:]...)
}

func tlsServerCertificateVerify(input []byte, options crypto.SignerOpts) bool {
	prefix := append(bytes.Repeat([]byte{32}, 64), "TLS 1.3, server CertificateVerify\x00"...)
	return options != nil && options.HashFunc() == 0 && (len(input) == len(prefix)+32 || len(input) == len(prefix)+48) && bytes.HasPrefix(input, prefix)
}

func (signer *serviceCertificateSigner) Sign(_ io.Reader, input []byte, options crypto.SignerOpts) ([]byte, error) {
	if signer == nil || signer.service == nil || !tlsServerCertificateVerify(input, options) {
		return nil, ErrUnavailable
	}
	service := signer.service
	if err := service.enter(); err != nil {
		return nil, err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := service.checkLocked(); err != nil {
		return nil, err
	}
	if len(service.lifetime.binding.private) != ed25519.PrivateKeySize {
		return nil, ErrUnavailable
	}
	signature := ed25519.Sign(service.lifetime.binding.private, input)
	if err := service.checkLocked(); err != nil {
		clear(signature)
		return nil, err
	}
	return signature, nil
}

// ProveInitial signs only a one-use coalesced request verified on this exact
// TLS Attachment after initial Continuity, under the live original Instance.
func (service *ServiceAttachment) ProveInitial(ctx context.Context, receipt *wire.InitialReceipt) (wire.Proof, error) {
	if ctx == nil || ctx.Err() != nil {
		return wire.Proof{}, ErrUnavailable
	}
	if err := service.enter(); err != nil {
		return wire.Proof{}, err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := service.checkLocked(); err != nil {
		return wire.Proof{}, err
	}
	if service.generation != 1 || service.proofIssued || service.evidence == nil || len(service.lifetime.binding.private) != ed25519.PrivateKeySize {
		return wire.Proof{}, ErrUnavailable
	}
	expected := wire.Challenge{Network: service.identity.Network, Target: service.identity.Target, InstanceGeneration: service.identity.InstanceGeneration, Context: service.logical}
	challenge, err := receipt.Consume(service.evidence, expected)
	if err != nil {
		return wire.Proof{}, err
	}
	digest, err := wire.ChallengeDigest(challenge)
	if err != nil {
		return wire.Proof{}, err
	}
	service.proofIssued = true
	proof := wire.Proof{ChallengeDigest: digest}
	copy(proof.Signature[:], ed25519.Sign(service.lifetime.binding.private, digest[:]))
	if err := service.checkLocked(); err != nil {
		return wire.Proof{}, err
	}
	if err := ctx.Err(); err != nil {
		return wire.Proof{}, err
	}
	service.initialProof = proof
	return proof, nil
}
