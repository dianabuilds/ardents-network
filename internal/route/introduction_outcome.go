package route

import (
	"errors"
	"time"
)

const (
	introductionSlotReadyKind      = 8
	introductionDeliveryResultKind = 9

	// IntroductionDelivered means the Introduction duty accepted exactly one
	// sealed record for forwarding on the live slot. It does not claim that a
	// Publisher opened a Responder leg or that a Service connection succeeded.
	IntroductionDelivered byte = 1
	// IntroductionUnavailable is the only v1 negative outcome. It covers a
	// missing, expired, drained, or already spent slot without disclosure.
	IntroductionUnavailable byte = 2
)

// IntroductionSlotReady is the Introduction duty's exact confirmation that a
// Publisher live slot is retained. It carries no Service or route material.
type IntroductionSlotReady struct {
	Reachability, JoinHandle [32]byte
	NotAfter                 time.Time
}

// IntroductionDeliveryResult gives the submitting User an explicit bounded
// C-2 outcome for its exact attachment. It does not expose why a slot was
// unavailable and never indicates Publisher or Service success.
type IntroductionDeliveryResult struct {
	AttachmentID [32]byte
	Outcome      byte
}

func EncodeIntroductionSlotReady(input IntroductionSlotReady) ([]byte, error) {
	if err := validIntroductionSlotReady(input); err != nil {
		return nil, err
	}
	body := make([]byte, 0, 2+1+1+len(Profile)+32+32+8)
	body = appendUint16(body, 1)
	body = append(body, introductionSlotReadyKind)
	body = appendProfile(body)
	body = append(body, input.Reachability[:]...)
	body = append(body, input.JoinHandle[:]...)
	body = appendUint64(body, uint64(input.NotAfter.Unix()))
	return routeEnvelope(body)
}

func DecodeIntroductionSlotReady(raw []byte) (IntroductionSlotReady, error) {
	reader, err := routeBody(raw, introductionSlotReadyKind)
	if err != nil {
		return IntroductionSlotReady{}, err
	}
	result := IntroductionSlotReady{}
	if result.Reachability, err = wireIdentifier(reader, "Introduction ready reachability"); err != nil {
		return IntroductionSlotReady{}, err
	}
	if result.JoinHandle, err = wireIdentifier(reader, "Introduction ready join handle"); err != nil {
		return IntroductionSlotReady{}, err
	}
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return IntroductionSlotReady{}, errors.New("Introduction slot ready has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validIntroductionSlotReady(result); err != nil {
		return IntroductionSlotReady{}, err
	}
	return result, nil
}

func EncodeIntroductionDeliveryResult(input IntroductionDeliveryResult) ([]byte, error) {
	if input.AttachmentID == [32]byte{} || (input.Outcome != IntroductionDelivered && input.Outcome != IntroductionUnavailable) {
		return nil, errors.New("Introduction delivery result is invalid")
	}
	body := make([]byte, 0, 2+1+1+len(Profile)+32+1)
	body = appendUint16(body, 1)
	body = append(body, introductionDeliveryResultKind)
	body = appendProfile(body)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.Outcome)
	return routeEnvelope(body)
}

func DecodeIntroductionDeliveryResult(raw []byte) (IntroductionDeliveryResult, error) {
	reader, err := routeBody(raw, introductionDeliveryResultKind)
	if err != nil {
		return IntroductionDeliveryResult{}, err
	}
	result := IntroductionDeliveryResult{}
	if result.AttachmentID, err = wireIdentifier(reader, "Introduction result attachment"); err != nil {
		return IntroductionDeliveryResult{}, err
	}
	result.Outcome = reader.uint8()
	if reader.off != len(reader.raw) || (result.Outcome != IntroductionDelivered && result.Outcome != IntroductionUnavailable) {
		return IntroductionDeliveryResult{}, errors.New("Introduction delivery result is malformed")
	}
	return result, nil
}

func validIntroductionSlotReady(input IntroductionSlotReady) error {
	if input.Reachability == [32]byte{} || input.JoinHandle == [32]byte{} || input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 ||
		!input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("Introduction slot ready is invalid")
	}
	return nil
}
