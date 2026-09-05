package route

import (
	"errors"
	"time"
)

const introductionSlotRegistrationKind = 7

// IntroductionSlotRegistration is the Publisher-to-Introduction declaration
// of one live opaque delivery slot. The matching private slot authorization is
// carried only by the already admitted EndpointTransitBinding, never here or
// in a Service descriptor.
type IntroductionSlotRegistration struct {
	Reachability, JoinHandle [32]byte
	NotAfter                 time.Time
}

// EncodeIntroductionSlotRegistration returns the canonical live-slot control
// record. It carries no Target, Credential, Instance key, plaintext, or
// endpoint literal.
func EncodeIntroductionSlotRegistration(input IntroductionSlotRegistration) ([]byte, error) {
	if err := validIntroductionSlotRegistration(input); err != nil {
		return nil, err
	}
	body := make([]byte, 0, 2+1+1+len(Profile)+32+32+8)
	body = appendUint16(body, routeWireVersion)
	body = append(body, introductionSlotRegistrationKind)
	body = appendProfile(body)
	body = append(body, input.Reachability[:]...)
	body = append(body, input.JoinHandle[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return routeEnvelope(body)
}

// DecodeIntroductionSlotRegistration rejects malformed or surplus bytes before
// an Introduction duty can reserve a slot.
func DecodeIntroductionSlotRegistration(raw []byte) (IntroductionSlotRegistration, error) {
	reader, err := routeBody(raw, introductionSlotRegistrationKind)
	if err != nil {
		return IntroductionSlotRegistration{}, err
	}
	result := IntroductionSlotRegistration{}
	if result.Reachability, err = wireIdentifier(reader, "introduction reachability"); err != nil {
		return IntroductionSlotRegistration{}, err
	}
	if result.JoinHandle, err = wireIdentifier(reader, "introduction join handle"); err != nil {
		return IntroductionSlotRegistration{}, err
	}
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return IntroductionSlotRegistration{}, errors.New("introduction slot registration has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validIntroductionSlotRegistration(result); err != nil {
		return IntroductionSlotRegistration{}, err
	}
	return result, nil
}

func validIntroductionSlotRegistration(input IntroductionSlotRegistration) error {
	if input.Reachability == [32]byte{} || input.JoinHandle == [32]byte{} || input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 ||
		!input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("introduction slot registration is invalid")
	}
	return nil
}
