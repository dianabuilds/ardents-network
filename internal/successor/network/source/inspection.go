package source

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"time"
)

// Inspection declares one exact authenticated contact, including its honest
// family. It is not an acquisition plan: it grants no State acceptance, retry,
// source-pair eligibility, independent-family supply or runtime authority.
type Inspection struct {
	Source            Source
	ClientCertificate tls.Certificate
	Authorities       map[[32]byte]ed25519.PublicKey
	VerificationClock func() time.Time
	NetworkID         [32]byte
	MaterialIndex     uint32
}

// InspectLatest performs exactly one bounded LATEST exchange using the same
// complete declaration, key separation, mTLS and framing checks as acquisition.
// It joins the original socket/callback before returning copied response bytes.
// The response remains unaccepted evidence; State owns signed authentication.
func InspectLatest(ctx context.Context, input Inspection) (Message, error) {
	if ctx == nil || input.VerificationClock == nil || input.NetworkID == [32]byte{} || input.MaterialIndex >= 64 || len(input.Authorities) < 1 || len(input.Authorities) > 16 {
		return Message{}, errors.New("source inspection declaration is invalid")
	}
	if ctx.Err() != nil {
		return Message{}, errors.Join(ctx.Err(), context.Cause(ctx))
	}
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize || sha256.Sum256(public) != id {
			return Message{}, errors.New("source inspection authority pin is invalid")
		}
	}
	if err := validateCredential(input.ClientCertificate, false); err != nil {
		return Message{}, err
	}
	clientDigest, err := certificateDigest(input.ClientCertificate)
	if err != nil || digestIsAuthority(clientDigest, input.Authorities) {
		return Message{}, errors.New("source inspection client key collides with Epoch authority")
	}
	credential := cloneCertificate(input.ClientCertificate)
	defer clear(credential.PrivateKey.(ed25519.PrivateKey))
	resolved, err := declaredClient(input.Source, credential, clientDigest, input.Authorities, input.VerificationClock)
	if err != nil {
		return Message{}, err
	}
	return fetch(ctx, resolved, Message{Operation: "latest", NetworkDigest: NetworkDigest(input.NetworkID), MaterialIndex: input.MaterialIndex})
}
