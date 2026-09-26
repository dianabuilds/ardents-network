package route

// This file is the sole remaining portion of the generation-2 Interactive
// Route wire grammar. The sealed Introduction v1 HPKE record and its exact
// envelope framing are retained byte-for-byte (ADR-0062 provenance) because
// the maintained Instance contract still embeds an IntroductionPublic key in
// fresh signed publication data (ADR-0034) and the positive command test
// inventory exercises this opening path. ADR-0093 deleted every other v2
// codec; the superseding decision for this grammar is tracked separately with
// the publication v1 Introduction instruction review.

import (
	"crypto/hpke"
	"errors"
	"fmt"
	"time"
)

const (
	routeWireMagic   = "ardents-interactive-route-v2\x00"
	routeWireVersion = uint16(2)
	// Profile is the exact retired native Interactive Route v2 wire profile.
	// Node keeps it only to refuse exact stale State records without side
	// effects (ADR-0093); no maintained composition accepts a v2 listener.
	Profile         = "ardents-interactive-route-v2"
	maximumWireBody = 4096
)

type wireReader struct {
	raw []byte
	off int
}

func (reader *wireReader) take(length int) []byte {
	if length < 0 || length > len(reader.raw)-reader.off {
		reader.off = len(reader.raw) + 1
		return nil
	}
	value := reader.raw[reader.off : reader.off+length]
	reader.off += length
	return value
}

func (reader *wireReader) uint8() byte {
	value := reader.take(1)
	if len(value) != 1 {
		return 0
	}
	return value[0]
}

func (reader *wireReader) uint16() uint16 {
	value := reader.take(2)
	if len(value) != 2 {
		return 0
	}
	return uint16(value[0])<<8 | uint16(value[1])
}

func (reader *wireReader) uint64() uint64 {
	value := reader.take(8)
	if len(value) != 8 {
		return 0
	}
	var result uint64
	for _, item := range value {
		result = result<<8 | uint64(item)
	}
	return result
}

func routeEnvelope(body []byte) ([]byte, error) {
	if len(body) == 0 || len(body) > maximumWireBody {
		return nil, errors.New("route wire body is outside its bound")
	}
	result := make([]byte, 0, len(routeWireMagic)+2+len(body))
	result = append(result, routeWireMagic...)
	result = appendUint16(result, uint16(len(body)))
	return append(result, body...), nil
}

func routeBody(raw []byte, kind byte) (*wireReader, error) {
	if len(raw) < len(routeWireMagic)+2 || string(raw[:len(routeWireMagic)]) != routeWireMagic {
		return nil, errors.New("route wire magic is invalid")
	}
	reader := &wireReader{raw: raw[len(routeWireMagic):]}
	length := int(reader.uint16())
	body := reader.take(length)
	if reader.off != len(reader.raw) || len(body) != length || length == 0 || length > maximumWireBody {
		return nil, errors.New("route wire length is invalid")
	}
	reader = &wireReader{raw: body}
	if reader.uint16() != routeWireVersion || reader.uint8() != kind {
		return nil, errors.New("route wire kind or version is invalid")
	}
	profileLength := int(reader.uint8())
	profile := string(reader.take(profileLength))
	if !validRouteProfile(profile) {
		return nil, errors.New("route wire profile is invalid")
	}
	return reader, nil
}

func appendUint16(destination []byte, value uint16) []byte {
	return append(destination, byte(value>>8), byte(value))
}

func appendUint64(destination []byte, value uint64) []byte {
	for shift := uint(56); ; shift -= 8 {
		destination = append(destination, byte(value>>shift))
		if shift == 0 {
			return destination
		}
	}
}

func appendProfile(destination []byte) []byte {
	return append(append(destination, byte(len(Profile))), Profile...)
}

func validRouteProfile(value string) bool {
	if value != Profile || len(value) == 0 || len(value) > 63 {
		return false
	}
	for _, item := range []byte(value) {
		if item < 0x21 || item > 0x7e {
			return false
		}
	}
	return true
}

func wireIdentifier(reader *wireReader, name string) ([32]byte, error) {
	var result [32]byte
	copy(result[:], reader.take(len(result)))
	if result == [32]byte{} {
		return [32]byte{}, fmt.Errorf("route wire %s is missing", name)
	}
	return result, nil
}

const (
	sealedIntroductionKind = 3
	encapsulationLength    = 32
	minimumCiphertext      = 16
)

// SealedIntroduction is the C-2 relay-visible context and its Service-only
// HPKE payload. The visible fields deliberately exclude Service Target and
// Instance material.
type SealedIntroduction struct {
	NetworkID, Digest                                  [32]byte
	Epoch                                              uint64
	IntroductionNodeID, RendezvousNodeID, Reachability [32]byte
	NotAfter                                           time.Time
	JoinHandle, EndpointHandshake                      [32]byte
	Enc, Ciphertext                                    []byte
}

// IntroductionRecipient performs the one fixed-purpose decapsulation needed
// by SealedIntroduction without exposing recipient key bytes.
type IntroductionRecipient interface {
	OpenIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte) ([]byte, error)
}

// EncodeSealedIntroduction returns the canonical transport record. It does
// not encrypt; SealIntroduction owns the HPKE context so all callers use the
// same AAD.
func EncodeSealedIntroduction(input SealedIntroduction) ([]byte, error) {
	if err := validSealedIntroduction(input, true); err != nil {
		return nil, err
	}
	body := introductionPrefix(input)
	body = appendUint16(body, uint16(len(input.Enc)))
	body = append(body, input.Enc...)
	body = appendUint16(body, uint16(len(input.Ciphertext)))
	body = append(body, input.Ciphertext...)
	return routeEnvelope(body)
}

// DecodeSealedIntroduction rejects non-canonical bounds before a recipient
// performs HPKE decapsulation.
func DecodeSealedIntroduction(raw []byte) (SealedIntroduction, error) {
	reader, err := routeBody(raw, sealedIntroductionKind)
	if err != nil {
		return SealedIntroduction{}, err
	}
	result := SealedIntroduction{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return SealedIntroduction{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return SealedIntroduction{}, err
	}
	if result.IntroductionNodeID, err = wireIdentifier(reader, "introduction node identifier"); err != nil {
		return SealedIntroduction{}, err
	}
	if result.RendezvousNodeID, err = wireIdentifier(reader, "rendezvous node identifier"); err != nil {
		return SealedIntroduction{}, err
	}
	if result.Reachability, err = wireIdentifier(reader, "rendezvous reachability"); err != nil {
		return SealedIntroduction{}, err
	}
	notAfter := reader.uint64()
	if notAfter > uint64(^uint64(0)>>1) {
		return SealedIntroduction{}, errors.New("sealed Introduction expiry is invalid")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if result.JoinHandle, err = wireIdentifier(reader, "join handle"); err != nil {
		return SealedIntroduction{}, err
	}
	if result.EndpointHandshake, err = wireIdentifier(reader, "endpoint handshake context"); err != nil {
		return SealedIntroduction{}, err
	}
	encLength := int(reader.uint16())
	result.Enc = append([]byte(nil), reader.take(encLength)...)
	ciphertextLength := int(reader.uint16())
	result.Ciphertext = append([]byte(nil), reader.take(ciphertextLength)...)
	if reader.off != len(reader.raw) {
		return SealedIntroduction{}, errors.New("sealed Introduction has surplus bytes")
	}
	if err := validSealedIntroduction(result, true); err != nil {
		return SealedIntroduction{}, err
	}
	return result, nil
}

// SealIntroduction encrypts the Service-only plaintext under the fixed v1
// HPKE suite. Its visible prefix is authenticated AAD, not relay authority.
func SealIntroduction(input SealedIntroduction, recipient hpke.PublicKey, plaintext []byte) (SealedIntroduction, error) {
	if recipient == nil || len(plaintext) == 0 || len(plaintext) > 4080 {
		return SealedIntroduction{}, errors.New("sealed Introduction encryption input is invalid")
	}
	if err := validSealedIntroduction(input, false); err != nil {
		return SealedIntroduction{}, err
	}
	enc, sender, err := hpke.NewSender(recipient, hpke.HKDFSHA256(), hpke.AES128GCM(), introductionInfo())
	if err != nil {
		return SealedIntroduction{}, err
	}
	ciphertext, err := sender.Seal(introductionAAD(input), plaintext)
	if err != nil {
		return SealedIntroduction{}, err
	}
	input.Enc, input.Ciphertext = enc, ciphertext
	if err := validSealedIntroduction(input, true); err != nil {
		return SealedIntroduction{}, err
	}
	return input, nil
}

// OpenSealedIntroduction authenticates the exact visible v1 header before
// returning Service-only plaintext to its recipient.
func OpenSealedIntroduction(input SealedIntroduction, recipient hpke.PrivateKey) ([]byte, error) {
	if recipient == nil {
		return nil, errors.New("sealed Introduction recipient is unavailable")
	}
	return OpenSealedIntroductionWith(input, hpkeIntroductionRecipient{private: recipient})
}

// OpenSealedIntroductionWith authenticates the exact visible v1 header and
// delegates only its fixed-purpose HPKE opening operation to an opaque
// recipient.
func OpenSealedIntroductionWith(input SealedIntroduction, recipient IntroductionRecipient) ([]byte, error) {
	if recipient == nil {
		return nil, errors.New("sealed Introduction recipient is unavailable")
	}
	if err := validSealedIntroduction(input, true); err != nil {
		return nil, err
	}
	return recipient.OpenIntroduction(input.Enc, introductionInfo(), introductionAAD(input), input.Ciphertext)
}

type hpkeIntroductionRecipient struct {
	private hpke.PrivateKey
}

func (recipient hpkeIntroductionRecipient) OpenIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte) ([]byte, error) {
	receiver, err := hpke.NewRecipient(encapsulation, recipient.private, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return receiver.Open(authenticatedHeader, ciphertext)
}

func introductionPrefix(input SealedIntroduction) []byte {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+32+32+8+32+32)
	body = appendUint16(body, routeWireVersion)
	body = append(body, sealedIntroductionKind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.IntroductionNodeID[:]...)
	body = append(body, input.RendezvousNodeID[:]...)
	body = append(body, input.Reachability[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	body = append(body, input.JoinHandle[:]...)
	body = append(body, input.EndpointHandshake[:]...)
	return body
}

func introductionAAD(input SealedIntroduction) []byte {
	return append([]byte(routeWireMagic), introductionPrefix(input)...)
}

func introductionInfo() []byte {
	return []byte("ardents-interactive-route-v2\x00sealed-introduction\x00")
}

func validSealedIntroduction(input SealedIntroduction, encrypted bool) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.Epoch == 0 ||
		input.IntroductionNodeID == [32]byte{} || input.RendezvousNodeID == [32]byte{} ||
		input.IntroductionNodeID == input.RendezvousNodeID || input.Reachability == [32]byte{} ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || input.JoinHandle == [32]byte{} ||
		input.EndpointHandshake == [32]byte{} || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("sealed Introduction header is invalid")
	}
	if !encrypted {
		if len(input.Enc) != 0 || len(input.Ciphertext) != 0 {
			return errors.New("sealed Introduction must not supply ciphertext before sealing")
		}
		return nil
	}
	if len(input.Enc) != encapsulationLength || len(input.Ciphertext) < minimumCiphertext ||
		len(input.Ciphertext) > maximumWireBody {
		return errors.New("sealed Introduction ciphertext is invalid")
	}
	return nil
}
