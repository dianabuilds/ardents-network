package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"time"

	"github.com/cloudflare/circl/blindsign/blindrsa"
	"github.com/dianabuilds/ardents-network/internal/network/state"
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

// ClosedTokenBatchConfig binds a volatile Endpoint batch to one accepted
// State profile, one offline permission, and one receiving-duty challenge.
type ClosedTokenBatchConfig struct {
	Profile    state.ClosedProfileView
	Context    ClosedTokenContext
	Permission Permission
	HolderKey  ed25519.PrivateKey
	Count      uint16
	Now        time.Time
}

// PendingClosedTokenBatch retains CIRCL's opaque blinding State only in the
// current Endpoint process. It must be discarded after a failed exchange or
// process loss; callers cannot serialize or recreate it.
type PendingClosedTokenBatch struct {
	request ClosedTokenBatchRequest
	raw     []byte
	client  blindrsa.Client
	states  []blindrsa.State
	inputs  [][]byte
}

// PrepareClosedTokenBatch creates one fresh 1..32-token issuance request.
// It admits no key, authority, class, window, or holder fact outside the
// authenticated State profile and permission.
func PrepareClosedTokenBatch(config ClosedTokenBatchConfig) (*PendingClosedTokenBatch, error) {
	key, err := validateClosedTokenBatchConfig(config)
	if err != nil {
		return nil, err
	}
	public, err := parseClosedTokenPublicKey(key.SPKI[:])
	if err != nil {
		return nil, err
	}
	client, err := blindrsa.NewClient(blindrsa.SHA384PSSDeterministic, public)
	if err != nil {
		return nil, errors.New("construct closed blind token client")
	}
	pending := &PendingClosedTokenBatch{client: client, states: make([]blindrsa.State, 0, config.Count), inputs: make([][]byte, 0, config.Count)}
	pending.request.Permission = config.Permission
	pending.request.Class = config.Context.Class
	pending.request.WindowStart = config.Context.WindowStart
	pending.request.SPKI = key.SPKI
	if _, err := rand.Read(pending.request.RequestID[:]); err != nil || pending.request.RequestID == [32]byte{} {
		pending.Discard()
		return nil, errors.New("draw closed token batch request ID")
	}
	keyID := sha256.Sum256(key.SPKI[:])
	for index := uint16(0); index < config.Count; index++ {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil || nonce == [32]byte{} {
			pending.Discard()
			return nil, errors.New("draw closed token nonce")
		}
		_, input, _, err := ClosedTokenChallenge(config.Context, key.SPKI[:], nonce)
		if err != nil {
			pending.Discard()
			return nil, err
		}
		prepared, err := client.Prepare(rand.Reader, input)
		if err != nil {
			pending.Discard()
			return nil, errors.New("prepare closed blind token input")
		}
		blinded, blindState, err := client.Blind(rand.Reader, prepared)
		if err != nil || len(blinded) != closedTokenBlindElementSize || !validClosedTokenElement(blinded, public) {
			pending.Discard()
			return nil, errors.New("blind closed token input")
		}
		request := make([]byte, closedTokenRequestSize)
		binary.BigEndian.PutUint16(request[:2], closedTokenType)
		request[2] = keyID[len(keyID)-1]
		copy(request[3:], blinded)
		pending.request.BlindedRequests = append(pending.request.BlindedRequests, request)
		pending.states = append(pending.states, blindState)
		pending.inputs = append(pending.inputs, input)
	}
	signature := ed25519.Sign(config.HolderKey, closedTokenBatchTranscript(pending.request))
	copy(pending.request.Signature[:], signature)
	pending.raw, err = EncodeClosedTokenBatch(pending.request)
	if err != nil {
		pending.Discard()
		return nil, err
	}
	return pending, nil
}

// Request returns a defensive copy of the exact signed batch for a
// same-process retry. It never exposes CIRCL's opaque blinding State.
func (pending *PendingClosedTokenBatch) Request() []byte {
	if pending == nil {
		return nil
	}
	return append([]byte(nil), pending.raw...)
}

// Discard removes the Endpoint's references to all volatile request inputs
// and CIRCL state. A later retry needs a newly provisioned right.
func (pending *PendingClosedTokenBatch) Discard() {
	if pending == nil {
		return
	}
	for index := range pending.inputs {
		clear(pending.inputs[index])
	}
	for index := range pending.request.BlindedRequests {
		clear(pending.request.BlindedRequests[index])
	}
	clear(pending.raw)
	pending.inputs = nil
	pending.states = nil
	pending.request.BlindedRequests = nil
	pending.raw = nil
}

// EncodeClosedTokenBatch returns the canonical signed issuance batch.
func EncodeClosedTokenBatch(request ClosedTokenBatchRequest) ([]byte, error) {
	if err := validateClosedTokenBatchRequest(request); err != nil {
		return nil, err
	}
	permission, err := EncodePermission(request.Permission)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 0, closedTokenBatchBaseSize()+len(request.BlindedRequests)*closedTokenRequestSize)
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

func validateClosedTokenBatchConfig(config ClosedTokenBatchConfig) (state.ClosedProfileTokenKey, error) {
	if config.Count < 1 || config.Count > maximumClosedTokenBatch || len(config.HolderKey) != ed25519.PrivateKeySize ||
		config.Now.IsZero() || config.Now != config.Now.UTC() || config.Context.ProfileDigest != config.Profile.Digest ||
		config.Context.IssuerNodeID != config.Profile.IssuerNodeID || config.Profile.IssuanceAuthorityKey == [32]byte{} ||
		config.Profile.IssuerDutyGeneration == 0 || int(config.Profile.TokenKeyCount) > len(config.Profile.TokenKeys) ||
		config.Context.WindowStart != config.Permission.NotBefore ||
		config.Context.Class < 1 || config.Context.Class > 3 || config.Permission.HolderKey != [32]byte(config.HolderKey.Public().(ed25519.PublicKey)) ||
		config.Permission.Maxima[config.Context.Class-1] < uint32(config.Count) ||
		VerifyPermission(config.Permission, ed25519.PublicKey(config.Profile.IssuanceAuthorityKey[:]), config.Context.NetworkID,
			config.Profile.IssuerNodeID, config.Profile.IssuerDutyGeneration, config.Now) != nil {
		return state.ClosedProfileTokenKey{}, errors.New("closed token batch configuration is invalid")
	}
	for index := 0; index < int(config.Profile.TokenKeyCount); index++ {
		key := config.Profile.TokenKeys[index]
		if key.WindowStart == config.Context.WindowStart && key.Class == config.Context.Class {
			if _, err := parseClosedTokenPublicKey(key.SPKI[:]); err != nil {
				return state.ClosedProfileTokenKey{}, err
			}
			return key, nil
		}
	}
	return state.ClosedProfileTokenKey{}, errors.New("closed token key is absent from State")
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
