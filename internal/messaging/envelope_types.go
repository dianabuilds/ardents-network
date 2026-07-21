package messaging

//go:generate protoc --proto_path=../.. --go_out=../.. --go_opt=paths=source_relative ../../internal/messaging/private.proto

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"time"

	identityapi "ardents/internal/identity"
)

const (
	MessageClassDiscoveryRecord    = MessageClass_DISCOVERY_RECORD
	MessageClassBlobFetchRequest   = MessageClass_BLOB_FETCH_REQUEST
	MessageClassBlobFetchResponse  = MessageClass_BLOB_FETCH_RESPONSE
	MessageClassCapabilityControl  = MessageClass_CAPABILITY_CONTROL
	MessageClassBlobReplicaControl = MessageClass_BLOB_REPLICA_CONTROL
)

type SealRequest struct {
	Capability     identityapi.ResolvedCapability
	Class          MessageClass
	PayloadVersion uint32
	Payload        []byte
	Signer         ed25519.PrivateKey
	IssuedAt       time.Time
	Lifetime       time.Duration
	Random         io.Reader
}

type SealedEnvelope struct {
	PubsubTopic  string
	ContentTopic string
	Payload      []byte
}

func (SealedEnvelope) String() string   { return "sealed-private-envelope[redacted]" }
func (SealedEnvelope) GoString() string { return "sealed-private-envelope[redacted]" }
func (sealed SealedEnvelope) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PubsubTopic string
		Size        int
	}{sealed.PubsubTopic, len(sealed.Payload)})
}

type OpenRequest struct {
	Capability   identityapi.ResolvedCapability
	PubsubTopic  string
	ContentTopic string
	Payload      []byte
	Authorizer   identityapi.CapabilitySenderAuthorizer
	Replay       ReplayLedger
	Now          time.Time
}

type OpenedMessage struct {
	Class          MessageClass
	PayloadVersion uint32
	Payload        []byte
	Sender         string
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

func (OpenedMessage) String() string   { return "opened-private-message[redacted]" }
func (OpenedMessage) GoString() string { return "opened-private-message[redacted]" }
func (opened OpenedMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Class          MessageClass
		PayloadVersion uint32
		IssuedAt       time.Time
		ExpiresAt      time.Time
	}{opened.Class, opened.PayloadVersion, opened.IssuedAt, opened.ExpiresAt})
}

type ReplayUse struct {
	CapabilityRef identityapi.CapabilityRef
	Generation    uint32
	MessageID     [16]byte
	ExpiresAt     time.Time
	Now           time.Time
}

type ReplayLedger interface {
	Admit(ReplayUse) error
}

func classProperties(class MessageClass) (identityapi.CapabilityScope, time.Duration, bool) {
	switch class {
	case MessageClassDiscoveryRecord:
		return identityapi.CapabilityRealmDiscovery, 15 * time.Minute, true
	case MessageClassBlobFetchRequest, MessageClassBlobFetchResponse:
		return identityapi.CapabilityDataExchange, 2 * time.Minute, true
	case MessageClassBlobReplicaControl:
		return identityapi.CapabilityDataExchange, 2 * time.Minute, true
	case MessageClassCapabilityControl:
		return identityapi.CapabilityControl, 15 * time.Minute, true
	default:
		return "", 0, false
	}
}
