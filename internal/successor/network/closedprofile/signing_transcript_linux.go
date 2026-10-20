//go:build linux

package closedprofile

import (
	"crypto/ed25519"
	"errors"
)

// SigningTranscript validates an exact unsigned ARDCPR03 body against the
// explicit public State context and returns its fixed-purpose signing message.
// It grants no State acceptance or private-key access. Authority is the pinned
// State signer, distinct from the Admission authority carried inside the body.
// The producer Interface follows its genuine Linux-only offline consumer;
// verification and the authoritative transcript encoding remain portable.
func SigningTranscript(body []byte, context Context) ([]byte, error) {
	if len(body)+ed25519.SignatureSize > MaxSize || len(context.Authority) != ed25519.PublicKeySize {
		return nil, errors.New("closed profile framing is invalid")
	}
	if _, err := parseBody(body, context.StateGeneration, context.NetworkID, context.EpochDigest, context.Epoch, context.Now.UTC()); err != nil {
		return nil, err
	}
	return signingTranscript(body), nil
}
