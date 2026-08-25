package publication

import (
	"encoding/binary"
	"errors"
)

const (
	introductionInstructionPrefix = "ardents-service-introduction-v1\x00"
	introductionInstructionSize   = len(introductionInstructionPrefix) + 2 + 32 + 8 + 32 + 32
)

// IntroductionInstruction is the Service-only HPKE plaintext that requests
// one Publisher-side responder attachment. It is never sent to a Route Node
// outside SealedIntroduction and is valid only for the exact live publication.
type IntroductionInstruction struct {
	Target, PublicationDigest, AttachmentID [32]byte
	Generation                              uint64
}

// EncodeIntroductionInstruction returns the closed v1 plaintext grammar for
// SealedIntroduction. It deliberately has no Node endpoint, route candidate,
// retry instruction, or application byte.
func EncodeIntroductionInstruction(input IntroductionInstruction) ([]byte, error) {
	if err := validIntroductionInstruction(input); err != nil {
		return nil, err
	}
	result := make([]byte, introductionInstructionSize)
	offset := copy(result, introductionInstructionPrefix)
	binary.BigEndian.PutUint16(result[offset:offset+2], 1)
	offset += 2
	copy(result[offset:offset+32], input.Target[:])
	offset += 32
	binary.BigEndian.PutUint64(result[offset:offset+8], input.Generation)
	offset += 8
	copy(result[offset:offset+32], input.PublicationDigest[:])
	offset += 32
	copy(result[offset:offset+32], input.AttachmentID[:])
	return result, nil
}

// DecodeIntroductionInstruction refuses any version, truncation, or surplus
// before a Publisher compares it with its current live publication.
func DecodeIntroductionInstruction(raw []byte) (IntroductionInstruction, error) {
	if len(raw) != introductionInstructionSize || string(raw[:len(introductionInstructionPrefix)]) != introductionInstructionPrefix ||
		binary.BigEndian.Uint16(raw[len(introductionInstructionPrefix):len(introductionInstructionPrefix)+2]) != 1 {
		return IntroductionInstruction{}, errors.New("service Introduction instruction encoding is invalid")
	}
	offset := len(introductionInstructionPrefix) + 2
	result := IntroductionInstruction{Generation: binary.BigEndian.Uint64(raw[offset+32 : offset+40])}
	copy(result.Target[:], raw[offset:offset+32])
	offset += 40
	copy(result.PublicationDigest[:], raw[offset:offset+32])
	offset += 32
	copy(result.AttachmentID[:], raw[offset:offset+32])
	if err := validIntroductionInstruction(result); err != nil {
		return IntroductionInstruction{}, err
	}
	return result, nil
}

// ValidateIntroductionInstruction requires the Service-only request to name
// precisely the current Publisher publication before it can open a responder
// attachment.
func (value Current) ValidateIntroductionInstruction(input IntroductionInstruction) error {
	if err := validIntroductionInstruction(input); err != nil {
		return err
	}
	if value.Credential.Target != input.Target || value.Credential.Generation != input.Generation || value.Digest != input.PublicationDigest {
		return errors.New("service Introduction instruction does not match the current publication")
	}
	return nil
}

func validIntroductionInstruction(input IntroductionInstruction) error {
	if input.Target == [32]byte{} || input.Generation == 0 || input.PublicationDigest == [32]byte{} || input.AttachmentID == [32]byte{} {
		return errors.New("service Introduction instruction is incomplete")
	}
	return nil
}
