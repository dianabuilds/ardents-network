package instance

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const stateSchema = "ardents-service-instance-root-v1"

type durableState struct {
	Phase               State
	NetworkID           [32]byte
	InstancePublic      [32]byte
	IntroductionPublic  [32]byte
	RequestCommitment   [32]byte
	InstancePrivate     ed25519.PrivateKey
	IntroductionPrivate []byte
	NotBefore           int64
	NotAfter            int64
	Response            []byte
	TerminalDigest      [32]byte
}

type storedState struct {
	Schema              string `json:"schema"`
	Phase               State  `json:"phase"`
	NetworkID           string `json:"network_id"`
	InstancePublic      string `json:"instance_public"`
	IntroductionPublic  string `json:"introduction_public"`
	RequestCommitment   string `json:"request_commitment"`
	InstancePrivate     string `json:"instance_private"`
	IntroductionPrivate string `json:"introduction_private"`
	NotBefore           int64  `json:"not_before"`
	NotAfter            int64  `json:"not_after"`
	Response            string `json:"response"`
	TerminalDigest      string `json:"terminal_digest"`
}

func (state durableState) present() bool { return state.Phase != "" }

func generateState(config InitializeConfig) (durableState, error) {
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return durableState{}, err
	}
	introductionPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		zero(instancePrivate)
		return durableState{}, err
	}
	state := durableState{Phase: StatePending, NetworkID: config.NetworkID, InstancePrivate: instancePrivate,
		IntroductionPrivate: introductionPrivate.Bytes(), NotBefore: config.NotBefore.Unix(), NotAfter: config.NotAfter.Unix()}
	copy(state.InstancePublic[:], instancePublic)
	copy(state.IntroductionPublic[:], introductionPrivate.PublicKey().Bytes())
	view := RequestView{NetworkID: state.NetworkID, InstancePublic: state.InstancePublic,
		IntroductionPublic: state.IntroductionPublic, NotBefore: state.NotBefore, NotAfter: state.NotAfter}
	state.RequestCommitment = requestCommitment(view)
	if err := state.validate(); err != nil {
		state.erase()
		return durableState{}, err
	}
	return state, nil
}

func (state durableState) requestView() (RequestView, error) {
	view := RequestView{NetworkID: state.NetworkID, InstancePublic: state.InstancePublic,
		IntroductionPublic: state.IntroductionPublic, NotBefore: state.NotBefore,
		NotAfter: state.NotAfter, Commitment: state.RequestCommitment}
	if _, err := ParseRequest(encodeRequest(view)); err != nil {
		return RequestView{}, ErrInvalid
	}
	if state.hasSecrets() && !state.secretsMatchPublic() {
		return RequestView{}, ErrInvalid
	}
	return view, nil
}

func (state durableState) validate() error {
	request, err := state.requestView()
	if err != nil {
		return err
	}
	responseValid := func() bool {
		response, parseErr := ParseResponse(state.Response)
		return parseErr == nil && response.RequestCommitment == request.Commitment &&
			credentialMatchesRequest(response.Credential, request)
	}
	switch state.Phase {
	case StatePending:
		if !state.hasSecrets() || len(state.Response) != 0 || state.TerminalDigest != ([32]byte{}) {
			return ErrInvalid
		}
	case StateAccepted:
		if !state.hasSecrets() || !responseValid() || state.TerminalDigest != ([32]byte{}) {
			return ErrInvalid
		}
	case StateConsumed:
		if !responseValid() || state.TerminalDigest != ([32]byte{}) {
			return ErrInvalid
		}
	case StateWithdrawn:
		if state.hasSecrets() || !responseValid() || state.TerminalDigest != ([32]byte{}) {
			return ErrInvalid
		}
	case StateRejected:
		if state.hasSecrets() || len(state.Response) != 0 || state.TerminalDigest == ([32]byte{}) {
			return ErrInvalid
		}
	case StateConflicting:
		if state.hasSecrets() || (len(state.Response) != 0 && !responseValid()) || state.TerminalDigest == ([32]byte{}) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (state durableState) hasSecrets() bool {
	return len(state.InstancePrivate) != 0 || len(state.IntroductionPrivate) != 0
}

func (state durableState) secretsMatchPublic() bool {
	if len(state.InstancePrivate) != ed25519.PrivateKeySize || len(state.IntroductionPrivate) != 32 {
		return false
	}
	instancePublic, ok := state.InstancePrivate.Public().(ed25519.PublicKey)
	introductionPrivate, err := ecdh.X25519().NewPrivateKey(state.IntroductionPrivate)
	return ok && bytes.Equal(instancePublic, state.InstancePublic[:]) && err == nil &&
		bytes.Equal(introductionPrivate.PublicKey().Bytes(), state.IntroductionPublic[:])
}

func (state durableState) credential() (publication.Credential, error) {
	response, err := ParseResponse(state.Response)
	if err != nil {
		return publication.Credential{}, ErrInvalid
	}
	return response.Credential, nil
}

func marshalState(state durableState) ([]byte, error) {
	if err := state.validate(); err != nil {
		return nil, err
	}
	stored := storedState{Schema: stateSchema, Phase: state.Phase,
		NetworkID: hex.EncodeToString(state.NetworkID[:]), InstancePublic: hex.EncodeToString(state.InstancePublic[:]),
		IntroductionPublic: hex.EncodeToString(state.IntroductionPublic[:]),
		RequestCommitment:  hex.EncodeToString(state.RequestCommitment[:]), NotBefore: state.NotBefore, NotAfter: state.NotAfter}
	if state.Phase == StatePending || state.Phase == StateAccepted {
		stored.InstancePrivate = base64.StdEncoding.EncodeToString(state.InstancePrivate)
		stored.IntroductionPrivate = base64.StdEncoding.EncodeToString(state.IntroductionPrivate)
	}
	if len(state.Response) != 0 {
		stored.Response = base64.StdEncoding.EncodeToString(state.Response)
	}
	if state.TerminalDigest != ([32]byte{}) {
		stored.TerminalDigest = hex.EncodeToString(state.TerminalDigest[:])
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func unmarshalState(raw []byte) (durableState, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return durableState{}, ErrInvalid
	}
	var stored storedState
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil {
		return durableState{}, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || stored.Schema != stateSchema {
		return durableState{}, ErrInvalid
	}
	state := durableState{Phase: stored.Phase, NotBefore: stored.NotBefore, NotAfter: stored.NotAfter}
	if err := decodeStateHex(stored.NetworkID, state.NetworkID[:], true); err != nil {
		return durableState{}, err
	}
	if stored.InstancePrivate != "" {
		state.InstancePrivate, _ = base64.StdEncoding.Strict().DecodeString(stored.InstancePrivate)
	}
	if stored.IntroductionPrivate != "" {
		state.IntroductionPrivate, _ = base64.StdEncoding.Strict().DecodeString(stored.IntroductionPrivate)
	}
	if stored.Response != "" {
		state.Response, _ = base64.StdEncoding.Strict().DecodeString(stored.Response)
	}
	if state.Phase == "" {
		state.Phase = StatePending
		if len(state.InstancePrivate) != ed25519.PrivateKeySize || len(state.IntroductionPrivate) != 32 {
			state.erase()
			return durableState{}, ErrInvalid
		}
		instancePublic, ok := state.InstancePrivate.Public().(ed25519.PublicKey)
		introductionPrivate, err := ecdh.X25519().NewPrivateKey(state.IntroductionPrivate)
		if !ok || err != nil {
			state.erase()
			return durableState{}, ErrInvalid
		}
		copy(state.InstancePublic[:], instancePublic)
		copy(state.IntroductionPublic[:], introductionPrivate.PublicKey().Bytes())
		view := RequestView{NetworkID: state.NetworkID, InstancePublic: state.InstancePublic,
			IntroductionPublic: state.IntroductionPublic, NotBefore: state.NotBefore, NotAfter: state.NotAfter}
		state.RequestCommitment = requestCommitment(view)
	} else {
		for _, item := range []struct {
			text string
			dest []byte
		}{{stored.InstancePublic, state.InstancePublic[:]}, {stored.IntroductionPublic, state.IntroductionPublic[:]},
			{stored.RequestCommitment, state.RequestCommitment[:]}} {
			if err := decodeStateHex(item.text, item.dest, true); err != nil {
				state.erase()
				return durableState{}, err
			}
		}
		if stored.TerminalDigest != "" {
			if err := decodeStateHex(stored.TerminalDigest, state.TerminalDigest[:], true); err != nil {
				state.erase()
				return durableState{}, err
			}
		}
	}
	if err := state.validate(); err != nil {
		state.erase()
		return durableState{}, err
	}
	return state, nil
}

func decodeStateHex(value string, destination []byte, nonzero bool) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(value) != len(destination)*2 || len(decoded) != len(destination) || hex.EncodeToString(decoded) != value {
		return ErrInvalid
	}
	copy(destination, decoded)
	if nonzero && bytes.Equal(destination, make([]byte, len(destination))) {
		return ErrInvalid
	}
	return nil
}

func (state *durableState) eraseSecrets() {
	zero(state.InstancePrivate)
	zero(state.IntroductionPrivate)
	state.InstancePrivate, state.IntroductionPrivate = nil, nil
}

func (state *durableState) erase() {
	state.eraseSecrets()
	zero(state.Response)
	*state = durableState{}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
