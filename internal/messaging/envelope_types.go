package messaging

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"time"

	identityapi "ardents/internal/identity"
	messagingprotocol "ardents/internal/messaging/protocol"
)

const (
	MessageClassDiscoveryRecord MessageClass = iota + 1
	MessageClassBlobFetchRequest
	MessageClassBlobFetchResponse
	MessageClassCapabilityControl
	MessageClassBlobReplicaControl
)

type MessageClass uint8

func messageClassToProtocol(class MessageClass) (messagingprotocol.MessageClass, bool) {
	switch class {
	case MessageClassDiscoveryRecord:
		return messagingprotocol.MessageClass_DISCOVERY_RECORD, true
	case MessageClassBlobFetchRequest:
		return messagingprotocol.MessageClass_BLOB_FETCH_REQUEST, true
	case MessageClassBlobFetchResponse:
		return messagingprotocol.MessageClass_BLOB_FETCH_RESPONSE, true
	case MessageClassCapabilityControl:
		return messagingprotocol.MessageClass_CAPABILITY_CONTROL, true
	case MessageClassBlobReplicaControl:
		return messagingprotocol.MessageClass_BLOB_REPLICA_CONTROL, true
	default:
		return messagingprotocol.MessageClass_MESSAGE_CLASS_UNSPECIFIED, false
	}
}

func messageClassFromProtocol(class messagingprotocol.MessageClass) (MessageClass, bool) {
	switch class {
	case messagingprotocol.MessageClass_DISCOVERY_RECORD:
		return MessageClassDiscoveryRecord, true
	case messagingprotocol.MessageClass_BLOB_FETCH_REQUEST:
		return MessageClassBlobFetchRequest, true
	case messagingprotocol.MessageClass_BLOB_FETCH_RESPONSE:
		return MessageClassBlobFetchResponse, true
	case messagingprotocol.MessageClass_CAPABILITY_CONTROL:
		return MessageClassCapabilityControl, true
	case messagingprotocol.MessageClass_BLOB_REPLICA_CONTROL:
		return MessageClassBlobReplicaControl, true
	default:
		return 0, false
	}
}

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
