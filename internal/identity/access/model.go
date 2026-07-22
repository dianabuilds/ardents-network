package access

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"time"

	identityprotocol "ardents/internal/identity/protocol"
)

type SourceKey [32]byte
type ChallengeID [16]byte
type SessionSecret [32]byte
type EnrollmentProof [32]byte

func (SessionSecret) String() string   { return "[redacted session secret]" }
func (SessionSecret) GoString() string { return "[redacted session secret]" }
func (SessionSecret) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted session secret]"))
}
func (SessionSecret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted session secret]"`), nil }
func (EnrollmentProof) String() string             { return "[redacted enrollment proof]" }
func (EnrollmentProof) GoString() string           { return "[redacted enrollment proof]" }
func (EnrollmentProof) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted enrollment proof]"))
}
func (EnrollmentProof) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted enrollment proof]"`), nil
}

type Audience struct {
	Node          string
	Interface     identityprotocol.Interface
	ProtocolMajor uint32
}

type AuthenticationBinding struct {
	Audience         Audience
	TransportProfile identityprotocol.TransportProfile
	PeerBinding      [32]byte
}

type Challenge struct {
	Version   uint32
	ID        ChallengeID
	Nonce     [32]byte
	Principal string
	Binding   AuthenticationBinding
	Purpose   identityprotocol.ChallengePurpose
	IssuedAt  time.Time
	ExpiresAt time.Time
}

func (c Challenge) String() string               { return "identity challenge [redacted]" }
func (c Challenge) GoString() string             { return c.String() }
func (c Challenge) MarshalJSON() ([]byte, error) { return []byte(`{"protected":"[redacted]"}`), nil }

type Session struct {
	ID           string
	Principal    string
	DeviceID     string
	CredentialID string
	Binding      AuthenticationBinding
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

func (s Session) String() string   { return fmt.Sprintf("identity session %s", s.ID) }
func (s Session) GoString() string { return s.String() }

type BeginRequest struct {
	Principal string
	Purpose   identityprotocol.ChallengePurpose
	Binding   AuthenticationBinding
	// Source is supplied only by the trusted listener adapter from OS peer
	// credentials (or its reduced shared-listener fallback), never by a client.
	Source SourceKey
}

type CompleteRequest struct {
	ChallengeID   ChallengeID
	Principal     string
	Binding       AuthenticationBinding
	Source        SourceKey
	RootPublicKey [32]byte
	Credential    []byte
	Signature     []byte
}

func (CompleteRequest) String() string   { return "identity completion request [redacted]" }
func (CompleteRequest) GoString() string { return "identity completion request [redacted]" }

type CompleteResult struct {
	Session         *Session
	SessionSecret   *SessionSecret
	EnrollmentProof *EnrollmentProof
}

func (r CompleteResult) String() string   { return "identity authentication result [redacted]" }
func (r CompleteResult) GoString() string { return r.String() }
func (r CompleteResult) MarshalJSON() ([]byte, error) {
	return []byte(`{"protected":"[redacted]"}`), nil
}

type AuditEvent struct {
	Outcome   string
	Reason    string
	Principal string
	DeviceID  string
	Audience  Audience
}

type AuditSink interface{ RecordIdentityAccess(AuditEvent) }

type AuditSinkFunc func(AuditEvent)

func (f AuditSinkFunc) RecordIdentityAccess(event AuditEvent) { f(event) }

var sessionIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func sessionID(lookup [sha256.Size]byte) string {
	return "s1_" + lowerASCII(sessionIDEncoding.EncodeToString(lookup[:]))
}

func lowerASCII(value string) string {
	raw := []byte(value)
	for index := range raw {
		if raw[index] >= 'A' && raw[index] <= 'Z' {
			raw[index] += 'a' - 'A'
		}
	}
	return string(raw)
}
