package custody

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"sort"
)

// AuthorityKind identifies the authority domain without revealing a Service
// Name or Service Target.
type AuthorityKind string

const (
	AuthorityService AuthorityKind = "service"
	AuthorityName    AuthorityKind = "name"
	// AuthorityAdmission is the separate closed-network offline permission
	// authority. It cannot sign State, Name, Service, or Release inputs.
	AuthorityAdmission AuthorityKind = "admission"
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

// authorityState exists only inside Custody. No public API accepts or returns it.
type authorityState struct {
	Binding          AuthorityBinding
	RootMaterial     []byte
	AdmissionJournal []byte
	ServiceJournal   []byte
	Generation       uint64
	Revision         uint64
	Watermarks       []Watermark
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
	Kind             AuthorityKind `json:"kind"`
	IDCommitment     string        `json:"id_commitment"`
	RootMaterial     string        `json:"root_material"`
	AdmissionJournal string        `json:"admission_journal,omitempty"`
	ServiceJournal   string        `json:"service_journal,omitempty"`
	Generation       uint64        `json:"generation"`
	Revision         uint64        `json:"revision"`
	Watermarks       []Watermark   `json:"watermarks"`
}

func encodeAuthorityState(purpose Purpose, state authorityState) ([]byte, error) {
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
			Kind:             state.Binding.Kind,
			IDCommitment:     hex.EncodeToString(state.Binding.IDCommitment[:]),
			RootMaterial:     base64.RawURLEncoding.EncodeToString(state.RootMaterial),
			AdmissionJournal: base64.RawURLEncoding.EncodeToString(state.AdmissionJournal),
			ServiceJournal:   base64.RawURLEncoding.EncodeToString(state.ServiceJournal),
			Generation:       state.Generation,
			Revision:         state.Revision,
			Watermarks:       append([]Watermark(nil), state.Watermarks...),
		},
	})
}

func decodeAuthorityState(raw []byte, expectedPurpose Purpose) (authorityState, error) {
	var encoded encodedState
	if err := decodeCanonical(raw, &encoded, maximumPlaintextBytes); err != nil {
		return authorityState{}, fmt.Errorf("state canonical: %w", ErrInvalid)
	}
	if encoded.Profile != "ardents-authority-state-v1" || encoded.SchemaVersion != 1 || encoded.Purpose != expectedPurpose {
		return authorityState{}, ErrInvalid
	}
	state := authorityState{
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
			return authorityState{}, ErrInvalid
		}
		copy(item.dest, decoded)
	}
	root, err := decodeRawURL(encoded.Authority.RootMaterial)
	if err != nil || len(root) == 0 || len(root) > maximumRootMaterialBytes {
		return authorityState{}, ErrInvalid
	}
	state.RootMaterial = root
	if encoded.Authority.AdmissionJournal != "" {
		journal, journalErr := decodeRawURL(encoded.Authority.AdmissionJournal)
		if journalErr != nil || len(journal) > allocation.MaximumJournalBytes {
			zero(root)
			return authorityState{}, ErrInvalid
		}
		state.AdmissionJournal = journal
	}
	if encoded.Authority.ServiceJournal != "" {
		journal, journalErr := decodeRawURL(encoded.Authority.ServiceJournal)
		if journalErr != nil || len(journal) > 1024 {
			zero(root)
			zero(state.AdmissionJournal)
			return authorityState{}, ErrInvalid
		}
		state.ServiceJournal = journal
	}
	if err := validateAuthorityState(state); err != nil {
		zero(root)
		zero(state.AdmissionJournal)
		zero(state.ServiceJournal)
		return authorityState{}, err
	}
	return state, nil
}

func validateAuthorityState(state authorityState) error {
	if state.Binding.Kind != AuthorityService && state.Binding.Kind != AuthorityName && state.Binding.Kind != AuthorityAdmission {
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
	if state.Binding.Kind == AuthorityAdmission {
		if len(state.ServiceJournal) != 0 {
			return ErrInvalid
		}
		return validateAdmissionAuthorityState(state)
	}
	if len(state.AdmissionJournal) != 0 {
		return ErrInvalid
	}
	if len(state.ServiceJournal) != 0 {
		request, err := publication.ParseServiceRequest(state.ServiceJournal)
		if err != nil || state.Binding.Kind != AuthorityService || request.NetworkID != state.Binding.Network {
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

func isZeroAuthorityState(state authorityState) bool {
	return state.Binding == (AuthorityBinding{}) && len(state.RootMaterial) == 0 && len(state.AdmissionJournal) == 0 && len(state.ServiceJournal) == 0 && state.Generation == 0 && state.Revision == 0 && len(state.Watermarks) == 0
}
