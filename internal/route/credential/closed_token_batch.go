package credential

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"math/big"
	"time"
)

const (
	closedTokenBatchMagic       = "ARDIBR01"
	closedTokenRequestSize      = 259
	closedTokenBlindElementSize = 256
	maximumClosedTokenBatch     = 32
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
	if len(raw) < closedTokenBatchBaseSize()+closedTokenRequestSize || len(raw) > maximumClosedTokenBatchSize || string(raw[:8]) != closedTokenBatchMagic {
		return ClosedTokenBatchRequest{}, errors.New("closed token batch framing is invalid")
	}
	offset := 8
	permission, err := DecodePermission(raw[offset : offset+permissionSize])
	if err != nil {
		return ClosedTokenBatchRequest{}, err
	}
	offset += permissionSize
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
	if count < 1 || count > maximumClosedTokenBatch || offset+count*closedTokenRequestSize+ed25519.SignatureSize != len(raw) {
		return ClosedTokenBatchRequest{}, errors.New("closed token batch count is invalid")
	}
	request.BlindedRequests = make([][]byte, 0, count)
	for index := 0; index < count; index++ {
		request.BlindedRequests = append(request.BlindedRequests, append([]byte(nil), raw[offset:offset+closedTokenRequestSize]...))
		offset += closedTokenRequestSize
	}
	copy(request.Signature[:], raw[offset:])
	if err := validateClosedTokenBatchRequest(request); err != nil {
		return ClosedTokenBatchRequest{}, err
	}
	return request, nil
}

func validateClosedTokenBatchRequest(request ClosedTokenBatchRequest) error {
	if request.RequestID == [32]byte{} || request.Class < 1 || request.Class > 3 || request.WindowStart.IsZero() ||
		request.WindowStart != request.WindowStart.UTC() || request.WindowStart.Truncate(time.Hour) != request.WindowStart ||
		len(request.BlindedRequests) < 1 || len(request.BlindedRequests) > maximumClosedTokenBatch ||
		request.Permission.Maxima[request.Class-1] < uint32(len(request.BlindedRequests)) {
		return errors.New("closed token batch facts are invalid")
	}
	public, err := parseClosedTokenPublicKey(request.SPKI[:])
	if err != nil {
		return err
	}
	keyID := sha256.Sum256(request.SPKI[:])
	for _, blinded := range request.BlindedRequests {
		if len(blinded) != closedTokenRequestSize || binary.BigEndian.Uint16(blinded[:2]) != closedTokenType || blinded[2] != keyID[len(keyID)-1] ||
			!validClosedTokenElement(blinded[3:], public) {
			return errors.New("closed token blinded request is invalid")
		}
	}
	if !ed25519.Verify(ed25519.PublicKey(request.Permission.HolderKey[:]), closedTokenBatchTranscript(request), request.Signature[:]) {
		return errors.New("closed token batch holder signature is invalid")
	}
	return nil
}

func closedTokenBatchTranscript(request ClosedTokenBatchRequest) []byte {
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

func validClosedTokenElement(encoded []byte, public *rsa.PublicKey) bool {
	if len(encoded) != closedTokenBlindElementSize || public == nil || public.N == nil {
		return false
	}
	value := new(big.Int).SetBytes(encoded)
	return value.Sign() > 0 && value.Cmp(public.N) < 0
}

func closedTokenBatchBaseSize() int {
	return len(closedTokenBatchMagic) + permissionSize + 32 + 1 + 8 + 346 + 2 + ed25519.SignatureSize
}
