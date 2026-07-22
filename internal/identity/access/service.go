package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrInvalidArgument   = errors.New("invalid identity authentication request")
	ErrUnauthenticated   = errors.New("authentication failed")
	ErrResourceExhausted = errors.New("identity authentication capacity exhausted")
	ErrUnavailable       = errors.New("identity authentication state unavailable")
	ErrInternal          = errors.New("identity authentication internal failure")
	ErrFeatureDisabled   = errors.New("identity access feature is disabled")
	ErrConflict          = errors.New("identity access state conflict")
)

type Clock interface{ Now() time.Time }
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Second) }

type Config struct {
	Database                    storage.Database
	Clock                       Clock
	Entropy                     io.Reader
	Audit                       AuditSink
	SessionLifetime             time.Duration
	EnableBootstrapTickets      bool
	GrantIssuer                 AccessGrantIssuer
	EnableApplicationEnrollment bool
}

type Service struct {
	clock                        Clock
	entropy                      io.Reader
	entropyMu                    sync.Mutex
	audit                        AuditSink
	challenges                   *challengeStore
	sessions                     *sessionStore
	proofs                       *proofStore
	revocations                  deviceRevocations
	enrollments                  enrollmentRepository
	grants                       grantRepository
	sessionLifetime              time.Duration
	deviceMu                     sync.Mutex
	bootstrapEnabled             bool
	grantIssuer                  AccessGrantIssuer
	applicationEnrollmentEnabled bool
}

func NewService(config Config) (*Service, error) {
	if config.Database == nil {
		return nil, fmt.Errorf("identity access database is required")
	}
	if config.Clock == nil {
		config.Clock = systemClock{}
	}
	if config.Entropy == nil {
		config.Entropy = rand.Reader
	}
	if config.SessionLifetime == 0 {
		config.SessionLifetime = identitycontract.DefaultSessionLifetime
	}
	if config.SessionLifetime <= 0 || config.SessionLifetime > identitycontract.MaxSessionLifetime {
		return nil, fmt.Errorf("invalid session lifetime")
	}
	var sessionKey, proofKey [32]byte
	if _, err := io.ReadFull(config.Entropy, sessionKey[:]); err != nil {
		return nil, fmt.Errorf("initialize ephemeral session key: %w", err)
	}
	if _, err := io.ReadFull(config.Entropy, proofKey[:]); err != nil {
		return nil, fmt.Errorf("initialize ephemeral proof key: %w", err)
	}
	return &Service{clock: config.Clock, entropy: config.Entropy, audit: config.Audit, challenges: newChallengeStore(), sessions: newSessionStore(sessionKey), proofs: newProofStore(proofKey), revocations: deviceRevocations{database: config.Database}, enrollments: enrollmentRepository{database: config.Database}, grants: grantRepository{database: config.Database}, sessionLifetime: config.SessionLifetime, bootstrapEnabled: config.EnableBootstrapTickets, grantIssuer: config.GrantIssuer, applicationEnrollmentEnabled: config.EnableApplicationEnrollment}, nil
}

func (s *Service) Begin(_ context.Context, request BeginRequest) (Challenge, error) {
	now := canonicalNow(s.clock.Now())
	if validateBegin(request, now) != nil {
		s.record("denied", "begin_invalid", "", "", Audience{})
		return Challenge{}, ErrInvalidArgument
	}
	var challenge Challenge
	for attempts := 0; attempts < 4; attempts++ {
		challenge = Challenge{Version: identitycontract.Version, Principal: request.Principal, Binding: request.Binding, Purpose: request.Purpose, IssuedAt: now, ExpiresAt: now.Add(identitycontract.ChallengeLifetime)}
		if s.random(challenge.ID[:]) != nil || s.random(challenge.Nonce[:]) != nil {
			s.record("denied", "entropy_unavailable", request.Principal, "", request.Binding.Audience)
			return Challenge{}, ErrInternal
		}
		err := s.challenges.add(now, storedChallenge{challenge: challenge, source: request.Source})
		if errors.Is(err, ErrInternal) {
			continue
		}
		if err != nil {
			s.record("denied", "challenge_capacity", request.Principal, "", request.Binding.Audience)
			return Challenge{}, err
		}
		s.record("accepted", "challenge_issued", request.Principal, "", request.Binding.Audience)
		return challenge, nil
	}
	s.record("denied", "challenge_collision", request.Principal, "", request.Binding.Audience)
	return Challenge{}, ErrInternal
}

func (s *Service) Complete(ctx context.Context, request CompleteRequest) (CompleteResult, error) {
	now := canonicalNow(s.clock.Now())
	stored, found := s.challenges.get(now, request.ChallengeID)
	if !found {
		return s.authenticationFailure("challenge_unknown", "", "", Audience{})
	}
	challenge := stored.challenge
	if request.Source != stored.source || request.Principal != challenge.Principal || request.Binding != challenge.Binding || validateChallenge(challenge, now) != nil {
		return s.authenticationFailure("challenge_binding_mismatch", challenge.Principal, "", challenge.Binding.Audience)
	}
	root := ed25519.PublicKey(request.RootPublicKey[:])
	principal, err := identityprincipal.FromEd25519PublicKey(root)
	if err != nil || principal.String() != request.Principal || principal.String() != challenge.Principal {
		return s.authenticationFailure("principal_mismatch", challenge.Principal, "", challenge.Binding.Audience)
	}
	signed, err := challengeSigningBytes(challenge)
	if err != nil {
		return s.authenticationFailure("challenge_invalid", request.Principal, "", request.Binding.Audience)
	}

	switch challenge.Purpose {
	case identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION:
		credential, parseErr := ParseAndVerifyKeyCredential(request.Credential, now)
		if parseErr != nil {
			return s.authenticationFailure("credential_invalid", request.Principal, "", request.Binding.Audience)
		}
		payload := credential.KeyCredentialPayload()
		if payload == nil || payload.Subject != request.Principal || !bytes.Equal(payload.RootPublicKey, root) || len(request.Signature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(payload.DevicePublicKey), signed, request.Signature) {
			return s.authenticationFailure("credential_binding_invalid", request.Principal, "", request.Binding.Audience)
		}
		s.deviceMu.Lock()
		defer s.deviceMu.Unlock()
		revoked, repositoryErr := s.revocations.revoked(ctx, request.Binding.Audience, request.Principal, payload.DeviceId)
		if repositoryErr != nil {
			s.record("denied", "store_unavailable", request.Principal, payload.DeviceId, request.Binding.Audience)
			return CompleteResult{}, ErrUnavailable
		}
		if revoked {
			s.sessions.invalidateDevice(payload.DeviceId)
			return s.authenticationFailure("device_revoked", request.Principal, payload.DeviceId, request.Binding.Audience)
		}
		if !s.challenges.consume(now, stored) {
			return s.authenticationFailure("challenge_replayed", request.Principal, payload.DeviceId, request.Binding.Audience)
		}
		return s.issueSession(now, stored.source, challenge, credential.ID(), payload.DeviceId)

	case identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF:
		if len(request.Credential) != 0 || len(request.Signature) != ed25519.SignatureSize || !ed25519.Verify(root, signed, request.Signature) {
			return s.authenticationFailure("enrollment_signature_invalid", request.Principal, "", request.Binding.Audience)
		}
		if !s.challenges.consume(now, stored) {
			return s.authenticationFailure("challenge_replayed", request.Principal, "", request.Binding.Audience)
		}
		return s.issueEnrollmentProof(now, challenge)
	default:
		return s.authenticationFailure("challenge_purpose_unknown", request.Principal, "", request.Binding.Audience)
	}
}

func (s *Service) issueSession(now time.Time, source SourceKey, challenge Challenge, credentialID, deviceID string) (CompleteResult, error) {
	for attempts := 0; attempts < 4; attempts++ {
		var secret SessionSecret
		if s.random(secret[:]) != nil {
			s.record("denied", "entropy_unavailable", challenge.Principal, deviceID, challenge.Binding.Audience)
			return CompleteResult{}, ErrInternal
		}
		lookup := s.sessions.lookup(secret)
		session := Session{ID: sessionID(lookup), Principal: challenge.Principal, DeviceID: deviceID, CredentialID: credentialID, Binding: challenge.Binding, IssuedAt: now, ExpiresAt: now.Add(s.sessionLifetime)}
		err := s.sessions.add(now, secret, session, source)
		if errors.Is(err, ErrInternal) {
			continue
		}
		if err != nil {
			s.record("denied", "session_capacity", challenge.Principal, deviceID, challenge.Binding.Audience)
			return CompleteResult{}, err
		}
		s.record("accepted", "session_issued", challenge.Principal, deviceID, challenge.Binding.Audience)
		return CompleteResult{Session: &session, SessionSecret: &secret}, nil
	}
	return CompleteResult{}, ErrInternal
}

func (s *Service) issueEnrollmentProof(now time.Time, challenge Challenge) (CompleteResult, error) {
	for attempts := 0; attempts < 4; attempts++ {
		var proof EnrollmentProof
		if s.random(proof[:]) != nil {
			return CompleteResult{}, ErrInternal
		}
		digest, err := challengeDigest(challenge)
		if err != nil {
			return CompleteResult{}, ErrInternal
		}
		err = s.proofs.add(now, proof, digest, challenge.ExpiresAt)
		if errors.Is(err, ErrInternal) {
			continue
		}
		if err != nil {
			return CompleteResult{}, err
		}
		s.record("accepted", "enrollment_proof_issued", challenge.Principal, "", challenge.Binding.Audience)
		return CompleteResult{EnrollmentProof: &proof}, nil
	}
	return CompleteResult{}, ErrInternal
}

func (s *Service) AuthenticateSession(ctx context.Context, secret SessionSecret, binding AuthenticationBinding) (Session, error) {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	now := canonicalNow(s.clock.Now())
	session, found := s.sessions.get(now, secret)
	if !found || session.Binding != binding {
		s.record("denied", "session_invalid", "", "", binding.Audience)
		return Session{}, ErrUnauthenticated
	}
	revoked, err := s.revocations.revoked(ctx, session.Binding.Audience, session.Principal, session.DeviceID)
	if err != nil {
		return Session{}, ErrUnavailable
	}
	if revoked {
		s.sessions.invalidateDevice(session.DeviceID)
		s.record("denied", "device_revoked", session.Principal, session.DeviceID, binding.Audience)
		return Session{}, ErrUnauthenticated
	}
	return session, nil
}

func (s *Service) recordDeviceRevocation(ctx context.Context, artifact *Artifact) error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	if err := s.revocations.record(ctx, artifact); err != nil {
		return err
	}
	payload := artifact.DeviceRevocationPayload()
	s.sessions.invalidateDevice(payload.TargetDeviceId)
	return nil
}

func (s *Service) consumeEnrollmentProof(proof EnrollmentProof, challenge Challenge) bool {
	digest, err := challengeDigest(challenge)
	if err != nil {
		return false
	}
	return s.proofs.consume(canonicalNow(s.clock.Now()), proof, digest)
}

func validateBegin(request BeginRequest, now time.Time) error {
	if now.IsZero() || request.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION && request.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(request.Principal); err != nil {
		return errInvalid
	}
	return validateAuthenticationBinding(request.Binding)
}

func validateAuthenticationBinding(binding AuthenticationBinding) error {
	if _, err := identityprincipal.Parse(binding.Audience.Node); err != nil || binding.Audience.ProtocolMajor != identitycontract.ProtocolMajor || binding.TransportProfile != identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1 {
		return errInvalid
	}
	if binding.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR && binding.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION {
		return errInvalid
	}
	if binding.PeerBinding == [32]byte{} {
		return errInvalid
	}
	return nil
}

func validateChallenge(challenge Challenge, now time.Time) error {
	if challenge.Version != identitycontract.Version || challenge.ID == (ChallengeID{}) || challenge.Nonce == [32]byte{} || validateAuthenticationBinding(challenge.Binding) != nil {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(challenge.Principal); err != nil {
		return errInvalid
	}
	if challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION && challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF {
		return errInvalid
	}
	if challenge.IssuedAt.Nanosecond() != 0 || challenge.ExpiresAt.Nanosecond() != 0 || challenge.ExpiresAt.Sub(challenge.IssuedAt) != identitycontract.ChallengeLifetime || now.Before(challenge.IssuedAt) || !now.Before(challenge.ExpiresAt) {
		return errInvalid
	}
	lower := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	if challenge.IssuedAt.Before(lower) || !challenge.IssuedAt.Before(upper) || !challenge.ExpiresAt.Before(upper) {
		return errInvalid
	}
	return nil
}

func challengeSigningBytes(challenge Challenge) ([]byte, error) {
	message := challengeProto(challenge)
	if validateChallenge(challenge, challenge.IssuedAt) != nil {
		return nil, errInvalid
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, errInvalid
	}
	domain := identitycontract.AuthenticationChallengeDomain
	if challenge.Purpose == identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF {
		domain = identitycontract.EnrollmentChallengeDomain
	} else if challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION {
		return nil, errInvalid
	}
	return append([]byte(domain), raw...), nil
}

// SignAuthenticationChallenge signs only a validated session challenge with
// the device key named by the supplied Key Credential. It is not a generic
// signing oracle.
func SignAuthenticationChallenge(challenge Challenge, credential *Artifact, device ed25519.PrivateKey) ([]byte, error) {
	payload := credential.KeyCredentialPayload()
	if challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION || len(device) != ed25519.PrivateKeySize || payload == nil || payload.GetSubject() != challenge.Principal || !bytes.Equal(payload.GetDevicePublicKey(), device.Public().(ed25519.PublicKey)) {
		return nil, errInvalid
	}
	raw, err := challengeSigningBytes(challenge)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(device, raw), nil
}

// SignEnrollmentChallenge signs only a validated enrollment-proof challenge
// for the Principal derived from the supplied root key.
func SignEnrollmentChallenge(challenge Challenge, root ed25519.PrivateKey) ([]byte, error) {
	if challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF || len(root) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	if err != nil || principal.String() != challenge.Principal {
		return nil, errInvalid
	}
	raw, err := challengeSigningBytes(challenge)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(root, raw), nil
}

func challengeProto(challenge Challenge) *identityprotocol.ChallengeFields {
	return &identityprotocol.ChallengeFields{Version: challenge.Version, Id: append([]byte(nil), challenge.ID[:]...), Nonce: append([]byte(nil), challenge.Nonce[:]...), Principal: challenge.Principal, Binding: &identityprotocol.AuthenticationBinding{Audience: &identityprotocol.Audience{Node: challenge.Binding.Audience.Node, Interface: challenge.Binding.Audience.Interface, ProtocolMajor: challenge.Binding.Audience.ProtocolMajor}, TransportProfile: challenge.Binding.TransportProfile, PeerBinding: append([]byte(nil), challenge.Binding.PeerBinding[:]...)}, Purpose: challenge.Purpose, IssuedAt: timestamppb.New(challenge.IssuedAt), ExpiresAt: timestamppb.New(challenge.ExpiresAt)}
}

// ChallengeFields returns an isolated wire representation of a validated
// challenge. Listener adapters use it to expose only the typed signing input.
func ChallengeFields(challenge Challenge) (*identityprotocol.ChallengeFields, error) {
	if validateChallenge(challenge, challenge.IssuedAt) != nil {
		return nil, ErrInvalidArgument
	}
	return challengeProto(challenge), nil
}

// ParseChallengeFields rejects incomplete, unknown, or non-canonical wire
// challenges before an administrative enrollment reaches durable state.
func ParseChallengeFields(fields *identityprotocol.ChallengeFields) (Challenge, error) {
	if fields == nil || hasUnknown(fields) || len(fields.Id) != len(ChallengeID{}) || len(fields.Nonce) != 32 || fields.Binding == nil || fields.Binding.Audience == nil || len(fields.Binding.PeerBinding) != 32 || fields.IssuedAt == nil || fields.ExpiresAt == nil || !fields.IssuedAt.IsValid() || !fields.ExpiresAt.IsValid() {
		return Challenge{}, ErrInvalidArgument
	}
	var id ChallengeID
	var nonce [32]byte
	var peer [32]byte
	copy(id[:], fields.Id)
	copy(nonce[:], fields.Nonce)
	copy(peer[:], fields.Binding.PeerBinding)
	challenge := Challenge{Version: fields.Version, ID: id, Nonce: nonce, Principal: fields.Principal, Binding: AuthenticationBinding{Audience: Audience{Node: fields.Binding.Audience.Node, Interface: fields.Binding.Audience.Interface, ProtocolMajor: fields.Binding.Audience.ProtocolMajor}, TransportProfile: fields.Binding.TransportProfile, PeerBinding: peer}, Purpose: fields.Purpose, IssuedAt: fields.IssuedAt.AsTime(), ExpiresAt: fields.ExpiresAt.AsTime()}
	if validateChallenge(challenge, challenge.IssuedAt) != nil || !proto.Equal(fields, challengeProto(challenge)) {
		return Challenge{}, ErrInvalidArgument
	}
	return challenge, nil
}

func challengeDigest(challenge Challenge) ([32]byte, error) {
	raw, err := challengeSigningBytes(challenge)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}
func canonicalNow(now time.Time) time.Time { return now.UTC().Truncate(time.Second) }
func (s *Service) random(destination []byte) error {
	s.entropyMu.Lock()
	defer s.entropyMu.Unlock()
	_, err := io.ReadFull(s.entropy, destination)
	return err
}
func (s *Service) record(outcome, reason, principal, deviceID string, audience Audience) {
	if s.audit != nil {
		s.audit.RecordIdentityAccess(AuditEvent{Outcome: outcome, Reason: reason, Principal: principal, DeviceID: deviceID, Audience: audience})
	}
}
func (s *Service) authenticationFailure(reason, principal, deviceID string, audience Audience) (CompleteResult, error) {
	s.record("denied", reason, principal, deviceID, audience)
	return CompleteResult{}, ErrUnauthenticated
}
