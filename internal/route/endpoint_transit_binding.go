package route

import (
	"errors"
	"time"
)

const (
	endpointTransitBindingKind  = 6
	maximumTransitAuthorization = 1024
)

// EndpointTransitBinding admits one Endpoint-to-Introduction or
// Endpoint-to-Responder TLS attempt. Authorization is an opaque, finite
// capability for the receiving duty; it is not an Entry Invite and carries no
// Target, Service material, or complete Route.
type EndpointTransitBinding struct {
	NetworkID, Digest, AttachmentID, TransitNodeID, ClientKeyDigest [32]byte
	Epoch                                                           uint64
	TransitRole                                                     byte
	NotAfter                                                        time.Time
	Authorization                                                   []byte
}

// EndpointTransitAdmission is the non-secret result of one receiving duty's
// atomic opaque-authorization and replay decision.
type EndpointTransitAdmission struct {
	AuthorizationID, NetworkID, Digest, TransitNodeID [32]byte
	Epoch                                             uint64
	TransitRole                                       byte
	NotAfter                                          time.Time
}

// EndpointTransitBindingAdmitter validates and consumes one opaque
// authorization for the exact TLS key, attachment, and transit duty. Its
// owner is the Introduction or Responder runtime, never Route.
type EndpointTransitBindingAdmitter func([]byte, [32]byte, [32]byte, byte, [32]byte, time.Time) (EndpointTransitAdmission, error)

// EncodeEndpointTransitBinding returns the sole canonical v1 binding for the
// C-2 Introduction and Publisher-side Responder first hops.
func EncodeEndpointTransitBinding(input EndpointTransitBinding) ([]byte, error) {
	if err := validEndpointTransitBinding(input); err != nil {
		return nil, err
	}
	body := endpointTransitBindingPrefix(input)
	body = appendUint16(body, uint16(len(input.Authorization)))
	body = append(body, input.Authorization...)
	return routeEnvelope(body)
}

// DecodeEndpointTransitBinding rejects malformed, unknown, surplus, or
// unauthorized-role bytes before a transit duty can allocate C-2 work.
func DecodeEndpointTransitBinding(raw []byte) (EndpointTransitBinding, error) {
	reader, err := routeBody(raw, endpointTransitBindingKind)
	if err != nil {
		return EndpointTransitBinding{}, err
	}
	result := EndpointTransitBinding{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return EndpointTransitBinding{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return EndpointTransitBinding{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return EndpointTransitBinding{}, err
	}
	result.TransitRole = reader.uint8()
	if result.TransitNodeID, err = wireIdentifier(reader, "transit node identifier"); err != nil {
		return EndpointTransitBinding{}, err
	}
	notAfter := reader.uint64()
	if notAfter > uint64(^uint64(0)>>1) {
		return EndpointTransitBinding{}, errors.New("endpoint transit binding expiry is invalid")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if result.ClientKeyDigest, err = wireIdentifier(reader, "client TLS key digest"); err != nil {
		return EndpointTransitBinding{}, err
	}
	authorizationLength := int(reader.uint16())
	result.Authorization = append([]byte(nil), reader.take(authorizationLength)...)
	if reader.off != len(reader.raw) {
		return EndpointTransitBinding{}, errors.New("endpoint transit binding has surplus bytes")
	}
	if err := validEndpointTransitBinding(result); err != nil {
		return EndpointTransitBinding{}, err
	}
	return result, nil
}

// AdmitEndpointTransitBinding verifies the presenting TLS peer and delegates
// opaque authorization plus tuple consumption to the receiving duty before it
// allocates any C-2 or Responder work.
func AdmitEndpointTransitBinding(input EndpointTransitBinding, peerClientKey [32]byte, now time.Time,
	admit EndpointTransitBindingAdmitter) error {
	if err := validEndpointTransitBinding(input); err != nil {
		return err
	}
	if peerClientKey == [32]byte{} || peerClientKey != input.ClientKeyDigest || now.IsZero() || !now.UTC().Before(input.NotAfter) || admit == nil {
		return errors.New("endpoint transit binding admission is invalid")
	}
	admission, err := admit(append([]byte(nil), input.Authorization...), input.AttachmentID, input.ClientKeyDigest,
		input.TransitRole, input.TransitNodeID, input.NotAfter)
	if err != nil {
		return err
	}
	if admission.AuthorizationID == [32]byte{} || admission.NetworkID != input.NetworkID || admission.Digest != input.Digest ||
		admission.Epoch != input.Epoch || admission.TransitRole != input.TransitRole || admission.TransitNodeID != input.TransitNodeID ||
		admission.NotAfter.IsZero() || input.NotAfter.After(admission.NotAfter) {
		return errors.New("endpoint transit binding does not match current authorization")
	}
	return nil
}

func endpointTransitBindingPrefix(input EndpointTransitBinding) []byte {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+1+32+8+32+2)
	body = appendUint16(body, 1)
	body = append(body, endpointTransitBindingKind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.TransitRole)
	body = append(body, input.TransitNodeID[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return append(body, input.ClientKeyDigest[:]...)
}

func validEndpointTransitBinding(input EndpointTransitBinding) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.TransitNodeID == [32]byte{} || input.ClientKeyDigest == [32]byte{} || input.Epoch == 0 ||
		(input.TransitRole != IntroductionRole && input.TransitRole != ResponderRole) || input.NotAfter.IsZero() ||
		input.NotAfter.Unix() <= 0 || len(input.Authorization) == 0 || len(input.Authorization) > maximumTransitAuthorization {
		return errors.New("endpoint transit binding is invalid")
	}
	return nil
}
