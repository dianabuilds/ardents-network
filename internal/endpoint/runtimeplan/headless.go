package runtimeplan

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Permission declares the local request/response paths and finite resource maxima.
type Permission struct {
	RequestPath  string    `json:"request_path"`
	ResponsePath string    `json:"response_path"`
	Maxima       [3]uint32 `json:"maxima"`
}

// Headless contains the local inputs for one Target Link participant runtime
// and its Connection Interface, without selecting a Service Target or Route.
type Headless struct {
	Role                string     `json:"role,omitempty"`
	TextTokenRoot       string     `json:"text_token_root,omitempty"`
	ReaderPermission    Permission `json:"reader_permission,omitempty"`
	PublisherPermission Permission `json:"publisher_permission,omitempty"`
	Schema              string     `json:"schema"`
	NetworkStateRoot    string     `json:"network_state_root"`
	// NetworkSourcePlan is an optional existing direct-Source plan. When it
	// is present, this runtime owns the State root's initial refresh and its
	// automatic refresh loop; a separate process cannot share that root lease.
	NetworkSourcePlan      string `json:"network_source_plan,omitempty"`
	EntryStateRoot         string `json:"entry_state_root"`
	TransitAcquisitionRoot string `json:"transit_acquisition_root"`
	ApplicationSocket      string `json:"application_socket"`
	AdministrationSocket   string `json:"administration_socket"`
	PublicationRoot        string `json:"publication_root"`
	ServiceInstanceRoot    string `json:"service_instance_root,omitempty"`
	// These three fields are decoded only so historical v1 input can receive
	// its bounded retirement refusal. There is no legacy runtime composition;
	// current v2 plans omit all three fields.
	AlphaCorpusStateRoot    string   `json:"alpha_corpus_state_root,omitempty"`
	LocalRoleStateRoot      string   `json:"local_role_state_root"`
	TimeConfidenceFile      string   `json:"time_confidence_file"`
	NetworkID               string   `json:"network_id"`
	NetworkAuthorities      []string `json:"network_authorities"`
	NetworkThreshold        int      `json:"network_threshold"`
	NetworkProfile          string   `json:"network_profile"`
	ClosedProfileAuthority  string   `json:"closed_profile_authority,omitempty"`
	AlphaCorpusAuthority    string   `json:"alpha_corpus_authority,omitempty"`
	AlphaCohort             string   `json:"alpha_cohort,omitempty"`
	BrokerID                string   `json:"broker_id"`
	ConnectionPrincipal     string   `json:"connection_principal"`
	AdministrationPrincipal string   `json:"administration_principal"`
	BytesEachDirection      uint32   `json:"bytes_each_direction"`
}

// DecodedHeadless projects checked declarations into pinned runtime identities.
type DecodedHeadless struct {
	Headless
	NetworkID, BrokerID, ConnectionPrincipal, AdministrationPrincipal [32]byte
	NetworkAuthorities                                                map[[32]byte]ed25519.PublicKey
	ClosedProfileAuthority                                            ed25519.PublicKey
}

// ErrHeadlessV1Retired identifies the unsupported historical runtime schema.
var ErrHeadlessV1Retired = errors.New("headless runtime plan v1 is retired")

// DecodeHeadless checks a bounded v2 local plan without acquiring runtime resources.
func DecodeHeadless(input []byte) (DecodedHeadless, error) {
	if len(input) == 0 || len(input) > 16<<10 {
		return DecodedHeadless{}, errors.New("headless runtime input exceeds its bound")
	}
	var raw Headless
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return DecodedHeadless{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return DecodedHeadless{}, errors.New("operator input contains trailing JSON")
	}
	if raw.Schema == "ardents-headless-runtime-v1" {
		return DecodedHeadless{}, ErrHeadlessV1Retired
	}
	if raw.Role != "" && raw.Role != "reader" {
		return DecodedHeadless{}, errors.New("text runtime role is unavailable")
	}
	if raw.Schema != "ardents-headless-runtime-v2" || raw.NetworkStateRoot == "" || raw.EntryStateRoot == "" ||
		raw.ApplicationSocket == "" || !filepath.IsAbs(raw.ApplicationSocket) ||
		raw.LocalRoleStateRoot == "" || raw.TimeConfidenceFile == "" || raw.NetworkProfile != carrier.ClosedRouteProfile || raw.BrokerID == "" ||
		raw.ConnectionPrincipal == "" || raw.Role == "" && (raw.AdministrationSocket == "" || !filepath.IsAbs(raw.AdministrationSocket) || raw.ApplicationSocket == raw.AdministrationSocket || raw.PublicationRoot == "" || raw.AdministrationPrincipal == "") {
		return DecodedHeadless{}, errors.New("headless runtime plan is incomplete")
	}
	if err := validateTextFields(raw); err != nil {
		return DecodedHeadless{}, err
	}
	result := DecodedHeadless{Headless: raw}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{raw.NetworkID, result.NetworkID[:]}, {raw.BrokerID, result.BrokerID[:]}, {raw.ConnectionPrincipal, result.ConnectionPrincipal[:]}} {
		if err := decodeFixedHex(field.encoded, field.destination); err != nil {
			return DecodedHeadless{}, err
		}
	}
	if raw.Role == "" {
		if err := decodeFixedHex(raw.AdministrationPrincipal, result.AdministrationPrincipal[:]); err != nil {
			return DecodedHeadless{}, err
		}
	}
	authorities, err := decodeAuthorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return DecodedHeadless{}, err
	}
	result.NetworkAuthorities = authorities
	authority := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if err := decodeFixedHex(raw.ClosedProfileAuthority, authority); err != nil {
		return DecodedHeadless{}, fmt.Errorf("text State profile authority: %w", err)
	}
	if _, pinned := authorities[sha256.Sum256(authority)]; !pinned {
		return DecodedHeadless{}, errors.New("text State profile authority is not pinned by State")
	}
	result.ClosedProfileAuthority = authority
	return result, nil
}

func decodeFixedHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != len(destination) {
		return fmt.Errorf("invalid fixed hexadecimal value")
	}
	copy(destination, decoded)
	return nil
}

func decodeAuthorities(encoded []string, maximum int) (map[[32]byte]ed25519.PublicKey, error) {
	if len(encoded) == 0 || len(encoded) > maximum {
		return nil, errors.New("authority key count is invalid")
	}
	values := make(map[[32]byte]ed25519.PublicKey, len(encoded))
	for _, value := range encoded {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeFixedHex(value, public); err != nil {
			return nil, err
		}
		values[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	return values, nil
}
