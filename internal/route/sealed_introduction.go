package route

import (
	"crypto/hpke"
	"errors"
	"time"
)

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
	if err := validSealedIntroduction(input, true); err != nil {
		return nil, err
	}
	receiver, err := hpke.NewRecipient(input.Enc, recipient, hpke.HKDFSHA256(), hpke.AES128GCM(), introductionInfo())
	if err != nil {
		return nil, err
	}
	return receiver.Open(introductionAAD(input), input.Ciphertext)
}

func introductionPrefix(input SealedIntroduction) []byte {
	body := make([]byte, 0, 2+1+1+len(routeProfile)+32+8+32+32+32+32+8+32+32)
	body = appendUint16(body, 1)
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
	return []byte("ardents-interactive-route-v1\x00sealed-introduction\x00")
}

func validSealedIntroduction(input SealedIntroduction, encrypted bool) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.Epoch == 0 ||
		input.IntroductionNodeID == [32]byte{} || input.RendezvousNodeID == [32]byte{} ||
		input.IntroductionNodeID == input.RendezvousNodeID || input.Reachability == [32]byte{} ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || input.JoinHandle == [32]byte{} ||
		input.EndpointHandshake == [32]byte{} {
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
