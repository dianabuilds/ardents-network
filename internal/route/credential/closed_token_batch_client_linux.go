//go:build linux

package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/cloudflare/circl/blindsign/blindrsa"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"time"
)

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
	pending := &PendingClosedTokenBatch{client: client, states: make([]blindrsa.State, 0, len(config.Contexts)), inputs: make([][]byte, 0, len(config.Contexts))}
	pending.request.Permission = config.Permission
	pending.request.Class = config.Contexts[0].Class
	pending.request.WindowStart = config.Contexts[0].WindowStart
	pending.request.SPKI = key.SPKI
	if _, err := rand.Read(pending.request.RequestID[:]); err != nil || pending.request.RequestID == [32]byte{} {
		pending.Discard()
		return nil, errors.New("draw closed token batch request ID")
	}
	keyID := sha256.Sum256(key.SPKI[:])
	for _, challenge := range config.Contexts {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil || nonce == [32]byte{} {
			pending.Discard()
			return nil, errors.New("draw closed token nonce")
		}
		_, input, _, err := ClosedTokenChallenge(challenge, key.SPKI[:], nonce)
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

// Finalize verifies every returned blind signature and returns RFC 9578
// token bytes only for a complete issued batch. It always drops the opaque
// CIRCL State, so a failed finalization cannot be retried after this call.
func (pending *PendingClosedTokenBatch) Finalize(result ClosedTokenBatchResult) ([][]byte, error) {
	if pending == nil || result.Status != ClosedTokenIssued || len(pending.states) == 0 || len(result.Signatures) != len(pending.states) {
		if pending != nil {
			pending.Discard()
		}
		return nil, errors.New("closed token batch is unavailable")
	}
	tokens := make([][]byte, 0, len(pending.states))
	for index, signature := range result.Signatures {
		if len(signature) != closedTokenBlindElementSize {
			pending.Discard()
			return nil, errors.New("closed token blind signature is invalid")
		}
		finalized, err := pending.client.Finalize(pending.states[index], signature)
		if err != nil || pending.client.Verify(pending.inputs[index], finalized) != nil {
			pending.Discard()
			return nil, errors.New("closed token signature verification failed")
		}
		token := make([]byte, 0, closedTokenSize)
		token = append(token, pending.inputs[index]...)
		token = append(token, finalized...)
		tokens = append(tokens, token)
	}
	pending.Discard()
	return tokens, nil
}

// FinalizeEncoded decodes the fixed issuer plaintext and verifies every token
// before returning it to the Endpoint-owned volatile stock.
func (pending *PendingClosedTokenBatch) FinalizeEncoded(raw []byte) ([][]byte, error) {
	result, err := DecodeClosedTokenBatchResult(raw)
	if err != nil {
		if pending != nil {
			pending.Discard()
		}
		return nil, err
	}
	return pending.Finalize(result)
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

func validateClosedTokenBatchConfig(config ClosedTokenBatchConfig) (state.ClosedProfileTokenKey, error) {
	if len(config.Contexts) < 1 || len(config.Contexts) > maximumClosedTokenBatch || len(config.HolderKey) != ed25519.PrivateKeySize ||
		config.Now.IsZero() || config.Now != config.Now.UTC() || config.Profile.IssuanceAuthorityKey == [32]byte{} ||
		config.Profile.IssuerDutyGeneration == 0 || int(config.Profile.TokenKeyCount) > len(config.Profile.TokenKeys) {
		return state.ClosedProfileTokenKey{}, errors.New("closed token batch configuration is invalid")
	}
	first := config.Contexts[0]
	for _, challenge := range config.Contexts {
		if challenge.NetworkID != config.Profile.NetworkID || challenge.ProfileDigest != config.Profile.Digest ||
			challenge.IssuerNodeID != config.Profile.IssuerNodeID || challenge.ReceiverNodeID == [32]byte{} || challenge.ReceiverDutyGeneration == 0 ||
			challenge.Class < 1 || challenge.Class > 3 || challenge.Class != first.Class || challenge.WindowStart != first.WindowStart ||
			challenge.WindowStart != config.Permission.NotBefore {
			return state.ClosedProfileTokenKey{}, errors.New("closed token batch receiving challenge is invalid")
		}
	}
	if config.Permission.HolderKey != [32]byte(config.HolderKey.Public().(ed25519.PublicKey)) ||
		config.Permission.Maxima[first.Class-1] < uint32(len(config.Contexts)) ||
		VerifyPermission(config.Permission, ed25519.PublicKey(config.Profile.IssuanceAuthorityKey[:]), config.Profile.NetworkID,
			config.Profile.IssuerNodeID, config.Profile.IssuerDutyGeneration, config.Now) != nil {
		return state.ClosedProfileTokenKey{}, errors.New("closed token batch permission is invalid")
	}
	var selected state.ClosedProfileTokenKey
	found := false
	for _, key := range config.Profile.TokenKeys[:config.Profile.TokenKeyCount] {
		if key.WindowStart == first.WindowStart && key.Class == first.Class {
			if found {
				return state.ClosedProfileTokenKey{}, errors.New("closed token key is ambiguous")
			}
			if _, err := parseClosedTokenPublicKey(key.SPKI[:]); err != nil {
				return state.ClosedProfileTokenKey{}, err
			}
			selected, found = key, true
		}
	}
	if !found {
		return state.ClosedProfileTokenKey{}, errors.New("closed token key is absent from State")
	}
	return selected, nil
}

// ClosedTokenBatchConfig binds a volatile Endpoint batch to one accepted
// State profile, one offline permission, and 1..32 ordered receiving challenges.
type ClosedTokenBatchConfig struct {
	Profile    state.ClosedProfileView
	Contexts   []ClosedTokenContext
	Permission Permission
	HolderKey  ed25519.PrivateKey
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
