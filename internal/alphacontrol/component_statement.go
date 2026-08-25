package alphacontrol

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const componentDomain = "ardents-alpha-control-component-v1\x00"

// ComponentStatement is a fixed component-local signature envelope. Its body
// is opaque to the catalog and must be interpreted only by that component's
// verifier. The verifying public key is deliberately absent: it is an
// independently pinned alpha-artifact companion, never catalog input.
type ComponentStatement struct {
	Class               ComponentClass
	Generation          uint64
	NotBefore, NotAfter time.Time
	Body                []byte
	Signature           [64]byte
}

// SignComponent returns one canonical signed component statement. It is used
// by controlled alpha fixture builders; production signing remains outside the
// reader and Endpoint.
func SignComponent(input ComponentStatement, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize || !validStatement(input) {
		return nil, errors.New("alpha control component statement is invalid")
	}
	payload := statementPayload(input)
	copy(input.Signature[:], ed25519.Sign(signer, append([]byte(componentDomain), payload...)))
	return append(payload, input.Signature[:]...), nil
}

// VerifyComponent verifies the component-local signature under the separately
// supplied component root and its exact catalog reference. It intentionally
// cannot alter Release, Network State, or Endpoint roots: it returns a
// reader-only classification.
func VerifyComponent(reference Component, raw []byte, root ed25519.PublicKey, at time.Time) Outcome {
	_, outcome := verifiedComponent(reference, raw, root, at)
	return outcome
}

func verifiedComponent(reference Component, raw []byte, root ed25519.PublicKey, at time.Time) (ComponentStatement, Outcome) {
	if len(raw) == 0 {
		return ComponentStatement{}, OutcomeUnavailable
	}
	if uint64(len(raw)) != uint64(reference.Size) || sha256.Sum256(raw) != reference.Digest || at.IsZero() || len(root) != ed25519.PublicKeySize {
		return ComponentStatement{}, OutcomeDigestMismatch
	}
	if len(raw) < 4+1+1+8+8+8+2+ed25519.SignatureSize {
		return ComponentStatement{}, OutcomeInvalid
	}
	payload, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	statement, err := decodeStatementPayload(payload)
	if err != nil || statement.Class != reference.Class || statement.Generation != reference.Generation ||
		statement.NotAfter != reference.NotAfter || sha256.Sum256(root) != reference.RootID ||
		!ed25519.Verify(root, append([]byte(componentDomain), payload...), signature) {
		return ComponentStatement{}, OutcomeInvalid
	}
	if at.Before(statement.NotBefore) || !at.Before(statement.NotAfter) {
		return ComponentStatement{}, OutcomeExpired
	}
	return statement, OutcomeAccepted
}

func statementPayload(input ComponentStatement) []byte {
	payload := make([]byte, 0, 4+1+1+8+8+8+4+len(input.Body))
	payload = append(payload, 'A', 'C', 'S', '1', 1, byte(input.Class))
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(input.Body)))
	return append(payload, input.Body...)
}

func decodeStatementPayload(payload []byte) (ComponentStatement, error) {
	if len(payload) < 4+1+1+8+8+8+4 || string(payload[:4]) != "ACS1" || payload[4] != 1 {
		return ComponentStatement{}, errors.New("alpha control component statement version is invalid")
	}
	class := ComponentClass(payload[5])
	if class < ComponentRelease || class > ComponentCompatibility {
		return ComponentStatement{}, errors.New("alpha control component statement class is invalid")
	}
	offset := 6
	statement := ComponentStatement{Class: class, Generation: binary.BigEndian.Uint64(payload[offset : offset+8])}
	offset += 8
	notBefore, notAfter := binary.BigEndian.Uint64(payload[offset:offset+8]), binary.BigEndian.Uint64(payload[offset+8:offset+16])
	offset += 16
	if notBefore > uint64(^uint64(0)>>1) || notAfter > uint64(^uint64(0)>>1) {
		return ComponentStatement{}, errors.New("alpha control component statement times are invalid")
	}
	statement.NotBefore, statement.NotAfter = time.Unix(int64(notBefore), 0).UTC(), time.Unix(int64(notAfter), 0).UTC()
	length := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
	offset += 4
	if length == 0 || offset+length != len(payload) {
		return ComponentStatement{}, errors.New("alpha control component statement body is invalid")
	}
	statement.Body = append([]byte(nil), payload[offset:]...)
	if !validStatement(statement) {
		return ComponentStatement{}, errors.New("alpha control component statement content is invalid")
	}
	return statement, nil
}

func validStatement(input ComponentStatement) bool {
	return input.Class >= ComponentRelease && input.Class <= ComponentCompatibility && input.Generation != 0 &&
		!input.NotBefore.IsZero() && !input.NotAfter.IsZero() && input.NotBefore.Before(input.NotAfter) &&
		input.NotBefore.Equal(input.NotBefore.UTC().Truncate(time.Second)) && input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) &&
		len(input.Body) > 0 && len(input.Body) <= 8<<20
}
