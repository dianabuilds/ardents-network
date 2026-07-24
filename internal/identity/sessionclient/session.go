// Package sessionclient owns the shared Principal-session client state machine,
// response validation, and retry policy used by Operator CLI and public SDK.
// It does not own credential custody or server-side session state.
package sessionclient

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
)

var ErrInvalidSessionHandshake = errors.New("invalid Principal session handshake")

// SessionHandshake owns the common Begin/Complete ordering and guarantees that
// each response is validated against time sampled after the corresponding RPC.
// Surface-specific transports, protobufs, signers, and error mappings remain in
// the callbacks.
type SessionHandshake[BeginResponse, Challenge, CompleteResponse, Session any] struct {
	Now              func() time.Time
	Begin            func(context.Context) (BeginResponse, error)
	AcceptChallenge  func(BeginResponse, time.Time) (Challenge, error)
	Complete         func(context.Context, Challenge) (CompleteResponse, error)
	AcceptCompletion func(CompleteResponse, time.Time) (Session, error)
}

func (h SessionHandshake[BeginResponse, Challenge, CompleteResponse, Session]) Run(ctx context.Context) (Session, error) {
	var zero Session
	if h.Now == nil || h.Begin == nil || h.AcceptChallenge == nil || h.Complete == nil || h.AcceptCompletion == nil {
		return zero, ErrInvalidSessionHandshake
	}
	begin, err := h.Begin(ctx)
	if err != nil {
		return zero, err
	}
	challenge, err := h.AcceptChallenge(begin, h.Now().UTC())
	if err != nil {
		return zero, err
	}
	complete, err := h.Complete(ctx, challenge)
	if err != nil {
		return zero, err
	}
	return h.AcceptCompletion(complete, h.Now().UTC())
}

type SessionCompletion struct {
	SessionSecret   []byte
	EnrollmentProof []byte
	SessionID       string
	ExpiresAt       time.Time
}

// ValidAuthenticationChallengeTimes defines the strict client-side challenge
// clock contract. Challenges receive no portable-artifact skew extension.
func ValidAuthenticationChallengeTimes(issuedAt, expiresAt, receivedAt time.Time) bool {
	issuedAt = issuedAt.UTC()
	expiresAt = expiresAt.UTC()
	receivedAt = receivedAt.UTC()
	if issuedAt.Nanosecond() != 0 || expiresAt.Nanosecond() != 0 ||
		expiresAt.Sub(issuedAt) != identitycontract.ChallengeLifetime ||
		issuedAt.Unix() < identitycontract.LowerTimestampUnix || issuedAt.Unix() >= identitycontract.UpperTimestampUnix ||
		expiresAt.Unix() >= identitycontract.UpperTimestampUnix {
		return false
	}
	return !receivedAt.Before(issuedAt) && receivedAt.Before(expiresAt)
}

// ValidSessionCompletion applies the version-1 response contract shared by
// Operator and Application clients. Protobuf validity and unknown fields are
// checked by the surface adapters before constructing this wire-neutral view.
func ValidSessionCompletion(completion SessionCompletion, receivedAt time.Time) bool {
	if len(completion.SessionSecret) != identitycontract.SessionSecretBytes ||
		len(completion.EnrollmentProof) != 0 ||
		!ValidSessionID(completion.SessionID) ||
		completion.ExpiresAt.Nanosecond() != 0 {
		return false
	}
	expiresAt := completion.ExpiresAt.UTC()
	if expiresAt.Unix() < identitycontract.LowerTimestampUnix || expiresAt.Unix() >= identitycontract.UpperTimestampUnix {
		return false
	}
	receivedAt = receivedAt.UTC()
	return receivedAt.Before(expiresAt) && !expiresAt.After(receivedAt.Add(identitycontract.MaxSessionLifetime))
}

// RetrySessionAuthentication reports whether a live waiter should retry after
// the singleflight leader failed only because the leader's context ended.
func RetrySessionAuthentication(waiterContext context.Context, leaderError error) bool {
	return waiterContext.Err() == nil &&
		(errors.Is(leaderError, context.Canceled) || errors.Is(leaderError, context.DeadlineExceeded))
}

func ValidSessionID(value string) bool {
	const prefix = "s1_"
	if len(value) != len(prefix)+52 || !strings.HasPrefix(value, prefix) {
		return false
	}
	suffix := value[len(prefix):]
	if suffix != strings.ToLower(suffix) {
		return false
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	raw, err := encoding.DecodeString(strings.ToUpper(suffix))
	if err != nil || len(raw) != 32 || strings.ToLower(encoding.EncodeToString(raw)) != suffix {
		return false
	}
	for _, item := range raw {
		if item != 0 {
			return true
		}
	}
	return false
}
