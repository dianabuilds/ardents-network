package identity

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChallengePurpose string

const (
	ChallengeSession         ChallengePurpose = "session"
	ChallengeEnrollmentProof ChallengePurpose = "enrollment_proof"
)

type TransportProfile string

const TransportUnixLocalV1 TransportProfile = "unix-local-v1"

type AuthenticationBinding struct {
	Audience         Audience
	TransportProfile TransportProfile
	PeerBinding      [32]byte
}
type Challenge struct {
	Version             uint32
	ID                  [16]byte
	Nonce               [32]byte
	Principal           string
	Binding             AuthenticationBinding
	Purpose             ChallengePurpose
	IssuedAt, ExpiresAt time.Time
}

func (Challenge) String() string   { return "identity challenge [redacted]" }
func (Challenge) GoString() string { return "identity challenge [redacted]" }

var ErrInvalidChallenge = errors.New("identity challenge is invalid")

func ValidateChallenge(challenge Challenge, now time.Time) error {
	if challenge.Version != identitycontract.Version || challenge.ID == [16]byte{} || challenge.Nonce == [32]byte{} || !validPrincipalID(challenge.Principal) || challenge.Binding.PeerBinding == [32]byte{} || challenge.Binding.TransportProfile != TransportUnixLocalV1 || challenge.Binding.Audience.ProtocolMajor != identitycontract.ProtocolMajor || !validPrincipalID(challenge.Binding.Audience.Node) {
		return ErrInvalidChallenge
	}
	if challenge.Binding.Audience.Interface != InterfaceOperator && challenge.Binding.Audience.Interface != InterfaceApplication {
		return ErrInvalidChallenge
	}
	if challenge.Purpose != ChallengeSession && challenge.Purpose != ChallengeEnrollmentProof || challenge.IssuedAt.Nanosecond() != 0 || challenge.ExpiresAt.Nanosecond() != 0 || challenge.ExpiresAt.Sub(challenge.IssuedAt) != identitycontract.ChallengeLifetime {
		return ErrInvalidChallenge
	}
	now = now.UTC()
	lower := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	if challenge.IssuedAt.Before(lower) || !challenge.IssuedAt.Before(upper) || !challenge.ExpiresAt.Before(upper) {
		return ErrInvalidChallenge
	}
	if now.Before(challenge.IssuedAt) || !now.Before(challenge.ExpiresAt) {
		return ErrInvalidChallenge
	}
	return nil
}

func SignAuthenticationChallenge(challenge Challenge, credential *Artifact, device ed25519.PrivateKey) ([]byte, error) {
	parsed := credential.KeyCredential()
	if challenge.Purpose != ChallengeSession || len(device) != ed25519.PrivateKeySize || parsed == nil || parsed.Subject != challenge.Principal || !bytes.Equal(parsed.DevicePublicKey, device.Public().(ed25519.PublicKey)) {
		return nil, ErrInvalidChallenge
	}
	raw, err := challengeBytes(challenge)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(device, raw), nil
}

func SignEnrollmentChallenge(challenge Challenge, root ed25519.PrivateKey) ([]byte, error) {
	if challenge.Purpose != ChallengeEnrollmentProof || len(root) != ed25519.PrivateKeySize || principalID(root.Public().(ed25519.PublicKey)) != challenge.Principal {
		return nil, ErrInvalidChallenge
	}
	raw, err := challengeBytes(challenge)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(root, raw), nil
}

func challengeBytes(challenge Challenge) ([]byte, error) {
	if ValidateChallenge(challenge, challenge.IssuedAt) != nil {
		return nil, ErrInvalidChallenge
	}
	var purpose identityv1.ChallengePurpose
	var profile identityv1.TransportProfile
	if challenge.Purpose == ChallengeSession {
		purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_SESSION
	} else {
		purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	}
	if challenge.Binding.TransportProfile == TransportUnixLocalV1 {
		profile = identityv1.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1
	}
	m := &identityv1.ChallengeFields{Version: challenge.Version, Id: append([]byte(nil), challenge.ID[:]...), Nonce: append([]byte(nil), challenge.Nonce[:]...), Principal: challenge.Principal, Binding: &identityv1.AuthenticationBinding{Audience: audienceToProto(challenge.Binding.Audience), TransportProfile: profile, PeerBinding: append([]byte(nil), challenge.Binding.PeerBinding[:]...)}, Purpose: purpose, IssuedAt: timestamppb.New(challenge.IssuedAt), ExpiresAt: timestamppb.New(challenge.ExpiresAt)}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return nil, ErrInvalidChallenge
	}
	domain := identitycontract.AuthenticationChallengeDomain
	if challenge.Purpose == ChallengeEnrollmentProof {
		domain = identitycontract.EnrollmentChallengeDomain
	}
	return append([]byte(domain), raw...), nil
}
