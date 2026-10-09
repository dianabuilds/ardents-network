package instance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

type State string

const (
	Pending     State = "pending"
	Accepted    State = "accepted"
	Consumed    State = "consumed"
	Withdrawn   State = "withdrawn"
	Rejected    State = "rejected"
	Conflicting State = "conflicting"
)

const stateSchema = "ardents-service-instance-root-v3"

type generationState struct {
	phase    State
	request  RequestView
	private  ed25519.PrivateKey
	response []byte
	terminal [32]byte
}

// Field names and order retain the selected persisted v3 grammar. Runtime roots
// remain independent; decoding this grammar is not automatic root adoption.
type storedState struct {
	Schema     string `json:"schema"`
	Phase      State  `json:"phase"`
	Network    string `json:"network_id"`
	Instance   string `json:"instance_public"`
	Commitment string `json:"request_commitment"`
	Private    string `json:"instance_private"`
	NotBefore  int64  `json:"not_before"`
	NotAfter   int64  `json:"not_after"`
	Response   string `json:"response"`
	Terminal   string `json:"terminal_digest"`
}

func generateState(network [32]byte, before, after int64) (generationState, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return generationState{}, err
	}
	value := generationState{phase: Pending, private: private, request: RequestView{NetworkID: network, NotBefore: before, NotAfter: after}}
	copy(value.request.InstancePublic[:], public)
	value.request.Commitment = requestCommitment(value.request)
	if err = value.validate(); err != nil {
		value.erase()
		return generationState{}, err
	}
	return value, nil
}

func (value generationState) validate() error {
	if _, err := ParseRequest(encodeRequest(value.request)); err != nil {
		return err
	}
	if len(value.private) != 0 {
		if len(value.private) != ed25519.PrivateKeySize || !bytes.Equal(value.private[32:], value.request.InstancePublic[:]) {
			return ErrInvalid
		}
		derived := ed25519.NewKeyFromSeed(value.private[:32])
		matches := bytes.Equal(derived, value.private)
		clear(derived)
		if !matches {
			return ErrInvalid
		}
	}
	hasResponse := len(value.response) != 0
	if hasResponse {
		if _, err := verifyResponse(value.response, value.request); err != nil {
			return ErrInvalid
		}
	}
	switch value.phase {
	case Pending:
		if len(value.private) == 0 || hasResponse || value.terminal != [32]byte{} {
			return ErrInvalid
		}
	case Accepted:
		if len(value.private) == 0 || !hasResponse || value.terminal != [32]byte{} {
			return ErrInvalid
		}
	case Consumed, Withdrawn:
		if len(value.private) != 0 || !hasResponse || value.terminal != [32]byte{} {
			return ErrInvalid
		}
	case Rejected:
		if len(value.private) != 0 || hasResponse || value.terminal == [32]byte{} {
			return ErrInvalid
		}
	case Conflicting:
		if len(value.private) != 0 || value.terminal == [32]byte{} {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func marshalState(value generationState) ([]byte, error) {
	if err := value.validate(); err != nil {
		return nil, err
	}
	stored := storedState{Schema: stateSchema, Phase: value.phase, Network: hex.EncodeToString(value.request.NetworkID[:]), Instance: hex.EncodeToString(value.request.InstancePublic[:]), Commitment: hex.EncodeToString(value.request.Commitment[:]), NotBefore: value.request.NotBefore, NotAfter: value.request.NotAfter}
	if len(value.private) != 0 {
		stored.Private = base64.StdEncoding.EncodeToString(value.private)
	}
	if len(value.response) != 0 {
		stored.Response = base64.StdEncoding.EncodeToString(value.response)
	}
	if value.terminal != [32]byte{} {
		stored.Terminal = hex.EncodeToString(value.terminal[:])
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func unmarshalState(raw []byte) (generationState, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return generationState{}, ErrInvalid
	}
	var stored storedState
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Schema != stateSchema {
		return generationState{}, ErrInvalid
	}
	value := generationState{phase: stored.Phase, request: RequestView{NotBefore: stored.NotBefore, NotAfter: stored.NotAfter}}
	fail := func() (generationState, error) { value.erase(); return generationState{}, ErrInvalid }
	for _, item := range []struct {
		text string
		dest []byte
	}{{stored.Network, value.request.NetworkID[:]}, {stored.Instance, value.request.InstancePublic[:]}, {stored.Commitment, value.request.Commitment[:]}} {
		decoded, err := hex.DecodeString(item.text)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != item.text {
			return fail()
		}
		copy(item.dest, decoded)
	}
	var err error
	if stored.Private != "" {
		value.private, err = base64.StdEncoding.Strict().DecodeString(stored.Private)
		if err != nil {
			return fail()
		}
	}
	if stored.Response != "" {
		value.response, err = base64.StdEncoding.Strict().DecodeString(stored.Response)
		if err != nil {
			return fail()
		}
	}
	if stored.Terminal != "" {
		decoded, err := hex.DecodeString(stored.Terminal)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != stored.Terminal {
			return fail()
		}
		copy(value.terminal[:], decoded)
	}
	canonical, err := marshalState(value)
	defer clear(canonical)
	if err != nil || !bytes.Equal(raw, canonical) {
		return fail()
	}
	return value, nil
}

func (value generationState) clone() generationState {
	value.private = append(ed25519.PrivateKey(nil), value.private...)
	value.response = append([]byte(nil), value.response...)
	return value
}

func (value *generationState) redact() { clear(value.private); value.private = nil }
func (value *generationState) erase() {
	value.redact()
	clear(value.response)
	*value = generationState{}
}
