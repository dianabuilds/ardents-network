package admission

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"math/big"
	"time"
)

const (
	closedTokenBatchMagic       = "ARDIBR01"
	closedTokenRequestSize      = 259
	closedTokenBlindElementSize = 256
	MaximumBatchTokens          = 32
	maximumClosedTokenBatchSize = 16 << 10
)

var closedTokenRequestDomain = []byte("ardents-issuance-request-v1\x00")

// ClosedTokenBatchRequest is the signed, bounded blind-token request sent to
// the State-selected issuer. It contains no Target, Endpoint identity, or
// application bytes.
type ClosedTokenBatchRequest struct {
	Permission      Permission
	RequestID       [32]byte
	Class           uint8
	WindowStart     time.Time
	SPKI            [346]byte
	BlindedRequests [][]byte
	Signature       [ed25519.SignatureSize]byte
}

// ClosedTokenBatchStatus is the small issuer result vocabulary. Encrypted
// carrier framing pads every value to the same response shape.
type ClosedTokenBatchStatus uint8

const (
	ClosedTokenIssued ClosedTokenBatchStatus = iota + 1
	ClosedTokenExhausted
	ClosedTokenWithdrawn
	ClosedTokenUnavailable
)

// ClosedTokenBatchResult carries blind signatures only for a committed
// issuance. It has no permission, holder, or application context.
type ClosedTokenBatchResult struct {
	Status     ClosedTokenBatchStatus
	Signatures [][]byte
}

// DecodeClosedTokenBatch verifies one exact signed issuance request before an
// issuer reserves a durable batch outcome.
func DecodeClosedTokenBatch(raw []byte) (ClosedTokenBatchRequest, error) {
	request, err := decodeBatch(raw)
	if err != nil {
		return ClosedTokenBatchRequest{}, err
	}
	if err := ValidateTokenBatchRequest(request); err != nil {
		return ClosedTokenBatchRequest{}, err
	}
	return request, nil
}

// InspectClosedTokenBatch preserves permission refusal categories while sharing
// the canonical parser and holder-proof validation with holder and issuer code.
func InspectClosedTokenBatch(ctx context.Context, raw []byte, facts Facts) (ClosedTokenBatchRequest, Outcome) {
	r, err := decodeBatch(raw)
	if err != nil {
		return ClosedTokenBatchRequest{}, Malformed
	}
	if facts.Class != r.Class || facts.Count != uint32(len(r.BlindedRequests)) {
		return ClosedTokenBatchRequest{}, Binding
	}
	if outcome := Inspect(ctx, raw[8:8+PermissionSize], facts); outcome != Accepted {
		return ClosedTokenBatchRequest{}, outcome
	}
	if err := ValidateTokenBatchRequest(r); err != nil {
		if errors.Is(err, ErrBatchSignature) {
			return ClosedTokenBatchRequest{}, Signature
		}
		return ClosedTokenBatchRequest{}, Malformed
	}
	return r, Accepted
}

func decodeBatch(raw []byte) (ClosedTokenBatchRequest, error) {
	if len(raw) < TokenBatchBaseSize()+closedTokenRequestSize || len(raw) > maximumClosedTokenBatchSize || string(raw[:8]) != closedTokenBatchMagic {
		return ClosedTokenBatchRequest{}, errors.New("closed token batch framing is invalid")
	}
	offset := 8
	permission, err := decodePermission(raw[offset:offset+PermissionSize], false)
	if err != nil {
		return ClosedTokenBatchRequest{}, err
	}
	offset += PermissionSize
	request := ClosedTokenBatchRequest{Permission: permission}
	copy(request.RequestID[:], raw[offset:offset+32])
	offset += 32
	request.Class = raw[offset]
	offset++
	request.WindowStart = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	copy(request.SPKI[:], raw[offset:offset+len(request.SPKI)])
	offset += len(request.SPKI)
	count := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if count < 1 || count > MaximumBatchTokens || offset+count*closedTokenRequestSize+ed25519.SignatureSize != len(raw) {
		return ClosedTokenBatchRequest{}, errors.New("closed token batch count is invalid")
	}
	request.BlindedRequests = make([][]byte, 0, count)
	for index := 0; index < count; index++ {
		request.BlindedRequests = append(request.BlindedRequests, append([]byte(nil), raw[offset:offset+closedTokenRequestSize]...))
		offset += closedTokenRequestSize
	}
	copy(request.Signature[:], raw[offset:])
	return request, nil
}

// ValidateTokenBatchRequest checks the canonical batch and holder proof.
// Permission authority and currentness are independently checked by the quota owner.
func ValidateTokenBatchRequest(request ClosedTokenBatchRequest) error {
	if request.Permission.Signature == [ed25519.SignatureSize]byte{} || request.RequestID == [32]byte{} || request.Class < 1 || request.Class > 3 || request.WindowStart.IsZero() || request.WindowStart != request.Permission.NotBefore ||
		request.WindowStart != request.WindowStart.UTC() || request.WindowStart.Truncate(time.Hour) != request.WindowStart ||
		len(request.BlindedRequests) < 1 || len(request.BlindedRequests) > MaximumBatchTokens ||
		request.Permission.Maxima[request.Class-1] < uint32(len(request.BlindedRequests)) {
		return errors.New("closed token batch facts are invalid")
	}
	public, err := parseBatchPublicKey(request.SPKI[:])
	if err != nil {
		return err
	}
	keyID := sha256.Sum256(request.SPKI[:])
	for _, blinded := range request.BlindedRequests {
		if len(blinded) != closedTokenRequestSize || binary.BigEndian.Uint16(blinded[:2]) != 2 || blinded[2] != keyID[len(keyID)-1] ||
			!ValidTokenElement(blinded[3:], public) {
			return errors.New("closed token blinded request is invalid")
		}
	}
	if !ed25519.Verify(ed25519.PublicKey(request.Permission.HolderKey[:]), TokenBatchTranscript(request), request.Signature[:]) {
		return ErrBatchSignature
	}
	return nil
}

// TokenBatchTranscript returns the exact holder-signature input.
func TokenBatchTranscript(request ClosedTokenBatchRequest) []byte {
	transcript := make([]byte, 0, len(closedTokenRequestDomain)+32+32+1+8+2+sha256.Size)
	transcript = append(transcript, closedTokenRequestDomain...)
	transcript = append(transcript, request.Permission.PermissionID[:]...)
	transcript = append(transcript, request.RequestID[:]...)
	transcript = append(transcript, request.Class)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(request.WindowStart.Unix()))
	transcript = binary.BigEndian.AppendUint16(transcript, uint16(len(request.BlindedRequests)))
	requests := make([]byte, 0, len(request.BlindedRequests)*closedTokenRequestSize)
	for _, blinded := range request.BlindedRequests {
		requests = append(requests, blinded...)
	}
	digest := sha256.Sum256(requests)
	clear(requests)
	return append(transcript, digest[:]...)
}

// ValidTokenElement checks the selected blind-request element's range.
func ValidTokenElement(encoded []byte, public *rsa.PublicKey) bool {
	if len(encoded) != closedTokenBlindElementSize || public == nil || public.N == nil {
		return false
	}
	value := new(big.Int).SetBytes(encoded)
	return value.Sign() > 0 && value.Cmp(public.N) < 0
}

// TokenBatchBaseSize is the framing width excluding blinded elements.
func TokenBatchBaseSize() int {
	return len(closedTokenBatchMagic) + PermissionSize + 32 + 1 + 8 + 346 + 2 + ed25519.SignatureSize
}

// BlindSignatureSize is the selected RSA blind-signature width.
const BlindSignatureSize = closedTokenBlindElementSize

// ErrBatchSignature distinguishes holder proof from malformed batch fields.
var ErrBatchSignature = errors.New("closed token batch holder signature is invalid")

func parseBatchPublicKey(raw []byte) (*rsa.PublicKey, error) {
	public, valid := issuerprofile.ParseKey(raw)
	if !valid {
		return nil, errors.New("closed token SPKI is not canonical")
	}
	return public, nil
}

// EncodeClosedTokenBatch returns the canonical signed issuance batch.
func EncodeClosedTokenBatch(request ClosedTokenBatchRequest) ([]byte, error) {
	if err := ValidateTokenBatchRequest(request); err != nil {
		return nil, err
	}
	permission, err := EncodePermission(request.Permission)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 0, TokenBatchBaseSize()+len(request.BlindedRequests)*closedTokenRequestSize)
	raw = append(raw, closedTokenBatchMagic...)
	raw = append(raw, permission...)
	raw = append(raw, request.RequestID[:]...)
	raw = append(raw, request.Class)
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.WindowStart.Unix()))
	raw = append(raw, request.SPKI[:]...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(request.BlindedRequests)))
	for _, blinded := range request.BlindedRequests {
		raw = append(raw, blinded...)
	}
	return append(raw, request.Signature[:]...), nil
}
