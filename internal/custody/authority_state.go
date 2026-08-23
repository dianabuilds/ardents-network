package custody

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
)

// AuthorityKind identifies the authority domain without revealing a Service
// Name or Service Target.
type AuthorityKind string

const (
	AuthorityService AuthorityKind = "service"
	AuthorityName    AuthorityKind = "name"
)

// AuthorityBinding is the expected public commitment set for one Authority.
// It is checked after successful envelope authentication before the record can
// be considered usable.
type AuthorityBinding struct {
	Environment  [32]byte
	Network      [32]byte
	Root         [32]byte
	Kind         AuthorityKind
	IDCommitment [32]byte
}

// Watermark is one monotonic authority fact. Domains are strictly sorted and
// unique ASCII strings in canonical state.
type Watermark struct {
	Domain string
	Value  uint64
}

// AuthorityState is supplied only to a creation operation. RootMaterial stays
// within custody and is never copied into a Receipt.
type AuthorityState struct {
	Binding      AuthorityBinding
	RootMaterial []byte
	Generation   uint64
	Revision     uint64
	Watermarks   []Watermark
}

type encodedState struct {
	Profile       string           `json:"profile"`
	SchemaVersion uint64           `json:"schema_version"`
	Purpose       Purpose          `json:"purpose"`
	Environment   string           `json:"environment"`
	Network       string           `json:"network"`
	Root          string           `json:"root"`
	Authority     encodedAuthority `json:"authority"`
}

type encodedAuthority struct {
	Kind         AuthorityKind `json:"kind"`
	IDCommitment string        `json:"id_commitment"`
	RootMaterial string        `json:"root_material"`
	Generation   uint64        `json:"generation"`
	Revision     uint64        `json:"revision"`
	Watermarks   []Watermark   `json:"watermarks"`
}

func encodeAuthorityState(purpose Purpose, state AuthorityState) ([]byte, error) {
	if err := validateAuthorityState(state); err != nil {
		return nil, err
	}
	return marshalCanonical(encodedState{
		Profile:       "ardents-authority-state-v1",
		SchemaVersion: 1,
		Purpose:       purpose,
		Environment:   hex.EncodeToString(state.Binding.Environment[:]),
		Network:       hex.EncodeToString(state.Binding.Network[:]),
		Root:          hex.EncodeToString(state.Binding.Root[:]),
		Authority: encodedAuthority{
			Kind:         state.Binding.Kind,
			IDCommitment: hex.EncodeToString(state.Binding.IDCommitment[:]),
			RootMaterial: base64.RawURLEncoding.EncodeToString(state.RootMaterial),
			Generation:   state.Generation,
			Revision:     state.Revision,
			Watermarks:   append([]Watermark(nil), state.Watermarks...),
		},
	})
}

func decodeAuthorityState(raw []byte, expectedPurpose Purpose) (AuthorityState, error) {
	var encoded encodedState
	if err := decodeCanonical(raw, &encoded, maximumPlaintextBytes); err != nil {
		return AuthorityState{}, fmt.Errorf("state canonical: %w", ErrInvalid)
	}
	if encoded.Profile != "ardents-authority-state-v1" || encoded.SchemaVersion != 1 || encoded.Purpose != expectedPurpose {
		return AuthorityState{}, ErrInvalid
	}
	state := AuthorityState{
		Binding:    AuthorityBinding{Kind: encoded.Authority.Kind},
		Generation: encoded.Authority.Generation,
		Revision:   encoded.Authority.Revision,
		Watermarks: append([]Watermark(nil), encoded.Authority.Watermarks...),
	}
	for _, item := range []struct {
		text string
		dest []byte
	}{
		{encoded.Environment, state.Binding.Environment[:]},
		{encoded.Network, state.Binding.Network[:]},
		{encoded.Root, state.Binding.Root[:]},
		{encoded.Authority.IDCommitment, state.Binding.IDCommitment[:]},
	} {
		decoded, err := hex.DecodeString(item.text)
		if len(item.text) != 64 || err != nil || len(decoded) != len(item.dest) || hex.EncodeToString(decoded) != item.text {
			return AuthorityState{}, ErrInvalid
		}
		copy(item.dest, decoded)
	}
	root, err := decodeRawURL(encoded.Authority.RootMaterial)
	if err != nil || len(root) == 0 || len(root) > maximumRootMaterialBytes {
		return AuthorityState{}, ErrInvalid
	}
	state.RootMaterial = root
	if err := validateAuthorityState(state); err != nil {
		zero(root)
		return AuthorityState{}, err
	}
	return state, nil
}

func validateAuthorityState(state AuthorityState) error {
	if state.Binding.Kind != AuthorityService && state.Binding.Kind != AuthorityName {
		return ErrInvalid
	}
	if len(state.RootMaterial) == 0 || len(state.RootMaterial) > maximumRootMaterialBytes {
		return ErrInvalid
	}
	if len(state.Watermarks) == 0 || len(state.Watermarks) > maximumWatermarks {
		return ErrInvalid
	}
	if !sort.SliceIsSorted(state.Watermarks, func(i, j int) bool {
		return state.Watermarks[i].Domain < state.Watermarks[j].Domain
	}) {
		return ErrInvalid
	}
	for index, watermark := range state.Watermarks {
		if len(watermark.Domain) == 0 || len(watermark.Domain) > maximumWatermarkDomainBytes || !isASCII(watermark.Domain) {
			return ErrInvalid
		}
		if index > 0 && state.Watermarks[index-1].Domain == watermark.Domain {
			return ErrInvalid
		}
	}
	return nil
}

func isASCII(value string) bool {
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func isZeroAuthorityState(state AuthorityState) bool {
	return state.Binding == (AuthorityBinding{}) && len(state.RootMaterial) == 0 && state.Generation == 0 && state.Revision == 0 && len(state.Watermarks) == 0
}
