package route

import (
	"errors"
	"io"
)

// IntroductionControlRecord is one closed first record over an admitted
// Introduction attachment. Registration and sealed submission are mutually
// exclusive; Raw preserves the exact canonical bytes for relay forwarding.
type IntroductionControlRecord struct {
	Registration *IntroductionSlotRegistration
	Sealed       *SealedIntroduction
	Raw          []byte
}

// WriteIntroductionSlotRegistration writes one Publisher live-slot declaration
// after its admitted Introduction TLS attachment.
func WriteIntroductionSlotRegistration(writer io.Writer, input IntroductionSlotRegistration) error {
	raw, err := EncodeIntroductionSlotRegistration(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// WriteSealedIntroduction writes one User sealed submission without exposing
// its Service-only plaintext to the Introduction duty.
func WriteSealedIntroduction(writer io.Writer, input SealedIntroduction) error {
	raw, err := EncodeSealedIntroduction(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadIntroductionControlRecord reads the one closed Publisher registration
// or User submission form. A sealed Raw value is the exact canonical byte
// sequence to forward on the live slot; the duty must never decrypt it.
func ReadIntroductionControlRecord(reader io.Reader) (IntroductionControlRecord, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return IntroductionControlRecord{}, err
	}
	if registration, err := DecodeIntroductionSlotRegistration(raw); err == nil {
		return IntroductionControlRecord{Registration: &registration, Raw: raw}, nil
	}
	if sealed, err := DecodeSealedIntroduction(raw); err == nil {
		return IntroductionControlRecord{Sealed: &sealed, Raw: raw}, nil
	}
	return IntroductionControlRecord{}, errors.New("introduction control record is not recognized")
}
