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
)

const stateSchema = "ardents-service-instance-root-v1"

type durableState struct {
	NetworkID           [32]byte
	InstancePrivate     ed25519.PrivateKey
	IntroductionPrivate []byte
	NotBefore           int64
	NotAfter            int64
}

type storedState struct {
	Schema              string `json:"schema"`
	NetworkID           string `json:"network_id"`
	InstancePrivate     string `json:"instance_private"`
	IntroductionPrivate string `json:"introduction_private"`
	NotBefore           int64  `json:"not_before"`
	NotAfter            int64  `json:"not_after"`
}

func (state durableState) present() bool { return len(state.InstancePrivate) != 0 }

func generateState(config InitializeConfig) (durableState, error) {
	_, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return durableState{}, err
	}
	introductionPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		zero(instancePrivate)
		return durableState{}, err
	}
	state := durableState{NetworkID: config.NetworkID, InstancePrivate: instancePrivate,
		IntroductionPrivate: introductionPrivate.Bytes(), NotBefore: config.NotBefore.Unix(), NotAfter: config.NotAfter.Unix()}
	if _, err := state.requestView(); err != nil {
		state.erase()
		return durableState{}, err
	}
	return state, nil
}

func (state durableState) requestView() (RequestView, error) {
	if state.NetworkID == ([32]byte{}) || len(state.InstancePrivate) != ed25519.PrivateKeySize ||
		len(state.IntroductionPrivate) != 32 || state.NotAfter <= state.NotBefore {
		return RequestView{}, ErrInvalid
	}
	instancePublic, ok := state.InstancePrivate.Public().(ed25519.PublicKey)
	if !ok || len(instancePublic) != 32 {
		return RequestView{}, ErrInvalid
	}
	introductionPrivate, err := ecdh.X25519().NewPrivateKey(state.IntroductionPrivate)
	if err != nil {
		return RequestView{}, ErrInvalid
	}
	view := RequestView{NetworkID: state.NetworkID, NotBefore: state.NotBefore, NotAfter: state.NotAfter}
	copy(view.InstancePublic[:], instancePublic)
	copy(view.IntroductionPublic[:], introductionPrivate.PublicKey().Bytes())
	view.Commitment = requestCommitment(view)
	return view, nil
}

func marshalState(state durableState) ([]byte, error) {
	if _, err := state.requestView(); err != nil {
		return nil, err
	}
	stored := storedState{Schema: stateSchema, NetworkID: hex.EncodeToString(state.NetworkID[:]),
		InstancePrivate:     base64.StdEncoding.EncodeToString(state.InstancePrivate),
		IntroductionPrivate: base64.StdEncoding.EncodeToString(state.IntroductionPrivate),
		NotBefore:           state.NotBefore, NotAfter: state.NotAfter}
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
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return durableState{}, ErrInvalid
	}
	if stored.Schema != stateSchema {
		return durableState{}, ErrInvalid
	}
	network, err := hex.DecodeString(stored.NetworkID)
	if err != nil || len(network) != 32 || stored.NetworkID != hex.EncodeToString(network) {
		return durableState{}, ErrInvalid
	}
	instancePrivate, err := base64.StdEncoding.Strict().DecodeString(stored.InstancePrivate)
	if err != nil || len(instancePrivate) != ed25519.PrivateKeySize {
		zero(instancePrivate)
		return durableState{}, ErrInvalid
	}
	introductionPrivate, err := base64.StdEncoding.Strict().DecodeString(stored.IntroductionPrivate)
	if err != nil || len(introductionPrivate) != 32 {
		zero(instancePrivate)
		zero(introductionPrivate)
		return durableState{}, ErrInvalid
	}
	state := durableState{InstancePrivate: ed25519.PrivateKey(instancePrivate), IntroductionPrivate: introductionPrivate,
		NotBefore: stored.NotBefore, NotAfter: stored.NotAfter}
	copy(state.NetworkID[:], network)
	if _, err := state.requestView(); err != nil {
		state.erase()
		return durableState{}, ErrInvalid
	}
	return state, nil
}

func (state *durableState) erase() {
	zero(state.InstancePrivate)
	zero(state.IntroductionPrivate)
	*state = durableState{}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
