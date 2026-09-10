//go:build linux

package connection

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
)

// InstanceAuthentication contains independently verified publication facts.
// Only the Publisher supplies its non-exporting Instance signer. These facts
// do not authorize a local Application or select a Route.
type InstanceAuthentication struct {
	Network, Target, Public [32]byte
	Generation              uint64
	Signer                  crypto.Signer
}

// NewAuthenticatedStream performs the generation-3 coalesced initial exchange
// on an already authenticated TLS Attachment. No Application bytes flow before
// both Instance and Continuity verification. NewStream retains the preceding
// sequential composition; neither constructor negotiates a record profile.
func NewAuthenticatedStream(input StreamConfig, identity InstanceAuthentication) (*Stream, error) {
	stream, err := NewStream(input)
	if err != nil {
		return nil, err
	}
	if err := validInitialAuthentication(input, identity); err != nil {
		return nil, err
	}
	verified, err := authenticateInitialStream(input.Context, input.Initial, input.ContinuityKey, input.Client, identity)
	if err != nil {
		stream.close()
		return nil, err
	}
	stream.initialAuthentication = verified
	stream.terminalReceipts = true
	return stream, nil
}

func validInitialAuthentication(input StreamConfig, identity InstanceAuthentication) error {
	if identity.Network == [32]byte{} || identity.Network != input.NetworkID || identity.Target == [32]byte{} ||
		identity.Public == [32]byte{} || identity.Generation == 0 || input.Initial.generation != 1 ||
		input.Initial.carrier == nil || input.Initial.context == [32]byte{} || input.Initial.exporterCommitment == [32]byte{} {
		return errors.New("initial Service identity is incomplete")
	}
	if input.Client {
		if identity.Signer != nil {
			return errors.New("client cannot carry an Instance signer")
		}
	} else {
		if identity.Signer == nil {
			return errors.New("local Publisher Instance signer is absent")
		}
		public, ok := identity.Signer.Public().(ed25519.PublicKey)
		if !ok || !bytes.Equal(public, identity.Public[:]) {
			return errors.New("local Publisher Instance signer does not match publication")
		}
	}
	return nil
}

func authenticateInitialStream(ctx context.Context, attachment *Attachment, key [32]byte, client bool, identity InstanceAuthentication) (_ *initialAuthentication, resultErr error) {
	bounded, cancel := context.WithTimeout(ctx, continuityExchangeLimit)
	defer cancel()
	// Cancellation also bounds carriers without deadline methods. Join the
	// actual transport-close callback before exposing a successful receipt.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() {
		defer close(interrupted)
		attachment.closeCarrier()
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
		resultErr = errors.Join(resultErr, bounded.Err())
	}()
	if err := bounded.Err(); err != nil {
		return nil, err
	}
	role, peerRole := RolePublisher, RoleClient
	if client {
		role, peerRole = RoleClient, RolePublisher
	}
	local, err := NewContinuity(key, role, 1, 0, 0, 0, attachment.context, attachment.exporterCommitment)
	if err != nil {
		return nil, err
	}
	var remote Record
	if client {
		remote, err = authenticateInitialClient(attachment, identity, local)
	} else {
		remote, err = authenticateInitialPublisher(attachment, identity, local, key)
	}
	if err != nil || remote.Continuity == nil ||
		VerifyContinuity(key, *remote.Continuity, peerRole, 1, attachment.context, attachment.exporterCommitment) != nil ||
		remote.Continuity.SendBase != 0 || remote.Continuity.SendEnd != 0 || remote.Continuity.ReceiveNext != 0 ||
		remote.Continuity.Nonce == local.Nonce {
		return nil, errors.Join(ErrActiveViolation, err)
	}
	return &initialAuthentication{attachment: attachment, peer: ContinuityPeer{
		LocalNonce: local.Nonce, PeerNonce: remote.Continuity.Nonce}}, nil
}

func authenticateInitialClient(attachment *Attachment, identity InstanceAuthentication, local Continuity) (Record, error) {
	challenge := Challenge{Network: identity.Network, Target: identity.Target,
		InstanceGeneration: identity.Generation, Context: attachment.context}
	if _, err := rand.Read(challenge.Nonce[:]); err != nil {
		return Record{}, err
	}
	// Only these two bounded records enter the initial request flight.
	var flight bytes.Buffer
	if err := Write(&flight, Record{Challenge: &challenge}); err != nil {
		return Record{}, err
	}
	if err := Write(&flight, Record{Continuity: &local}); err != nil {
		return Record{}, err
	}
	if _, err := flight.WriteTo(attachment.carrier); err != nil {
		return Record{}, err
	}
	record, err := Read(attachment.carrier)
	if err != nil {
		return Record{}, err
	}
	digest, err := ChallengeDigest(challenge)
	if err != nil || record.Proof == nil || record.Proof.ChallengeDigest != digest ||
		!ed25519.Verify(ed25519.PublicKey(identity.Public[:]), digest[:], record.Proof.Signature[:]) {
		return Record{}, ErrActiveViolation
	}
	return Read(attachment.carrier)
}

func authenticateInitialPublisher(attachment *Attachment, identity InstanceAuthentication, local Continuity, key [32]byte) (Record, error) {
	record, err := Read(attachment.carrier)
	if err != nil {
		return Record{}, err
	}
	challenge := record.Challenge
	if challenge == nil || challenge.Network != identity.Network || challenge.Target != identity.Target ||
		challenge.InstanceGeneration != identity.Generation || challenge.Context != attachment.context {
		return Record{}, ErrActiveViolation
	}
	digest, err := ChallengeDigest(*challenge)
	if err != nil {
		return Record{}, err
	}
	remote, err := Read(attachment.carrier)
	// Never sign or answer an unauthenticated initial Continuity.
	if err != nil || remote.Continuity == nil ||
		VerifyContinuity(key, *remote.Continuity, RoleClient, 1, attachment.context, attachment.exporterCommitment) != nil ||
		remote.Continuity.SendBase != 0 || remote.Continuity.SendEnd != 0 || remote.Continuity.ReceiveNext != 0 ||
		remote.Continuity.Nonce == local.Nonce {
		return Record{}, errors.Join(ErrActiveViolation, err)
	}
	signature, err := identity.Signer.Sign(rand.Reader, digest[:], crypto.Hash(0))
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(ed25519.PublicKey(identity.Public[:]), digest[:], signature) {
		return Record{}, errors.New("local Instance signer cannot prove this connection")
	}
	proof := Proof{ChallengeDigest: digest}
	copy(proof.Signature[:], signature)
	var flight bytes.Buffer
	if err := Write(&flight, Record{Proof: &proof}); err != nil {
		return Record{}, err
	}
	if err := Write(&flight, Record{Continuity: &local}); err != nil {
		return Record{}, err
	}
	_, err = flight.WriteTo(attachment.carrier)
	return remote, err
}
