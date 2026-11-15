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
	posixpath "path"
	"time"
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

// errHeadlessV1Retired identifies the unsupported historical runtime schema.
var errHeadlessV1Retired = errors.New("headless runtime plan v1 is retired")

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
		return DecodedHeadless{}, errHeadlessV1Retired
	}
	if raw.Role != "" && raw.Role != "reader" {
		return DecodedHeadless{}, errors.New("text runtime role is unavailable")
	}
	if raw.Schema != "ardents-headless-runtime-v2" || raw.NetworkStateRoot == "" || raw.EntryStateRoot == "" ||
		raw.ApplicationSocket == "" || !posixpath.IsAbs(raw.ApplicationSocket) ||
		raw.LocalRoleStateRoot == "" || raw.TimeConfidenceFile == "" || raw.NetworkProfile != "ardents-route-v3" || raw.BrokerID == "" ||
		raw.ConnectionPrincipal == "" || raw.Role == "" && (raw.AdministrationSocket == "" || !posixpath.IsAbs(raw.AdministrationSocket) || raw.ApplicationSocket == raw.AdministrationSocket || raw.PublicationRoot == "" || raw.AdministrationPrincipal == "") {
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
	if raw.NetworkThreshold < 1 || raw.NetworkThreshold > len(authorities) {
		return DecodedHeadless{}, errors.New("invalid declared threshold")
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
		identity := sha256.Sum256(public)
		if _, duplicate := values[identity]; duplicate {
			return nil, errors.New("duplicate declared authority key")
		}
		values[identity] = ed25519.PublicKey(public)
	}
	return values, nil
}

// Source declares an existing direct-Source configuration and credential paths.
// Decoding declarations never reads the referenced credentials.
type Source struct {
	Schema               string         `json:"schema"`
	NetworkID            string         `json:"network_id"`
	AuthorityPublic      []string       `json:"authority_public"`
	Threshold            int            `json:"threshold"`
	ClockObservedAt      string         `json:"clock_observed_at"`
	ClockObservationFile string         `json:"clock_observation_file,omitempty"`
	OrderSeed            string         `json:"order_seed"`
	MaterializationIndex uint32         `json:"materialization_index"`
	RefreshIntervalMS    uint32         `json:"refresh_interval_ms,omitempty"`
	RuntimeProfile       string         `json:"runtime_profile,omitempty"`
	LocalRoleStateRoot   string         `json:"local_role_state_root"`
	ClientCertificate    string         `json:"client_certificate"`
	ClientKey            string         `json:"client_key"`
	Sources              []SourceMember `json:"sources"`
}

// SourceMember declares one pinned direct-Source endpoint.
type SourceMember struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

// DecodedSource contains checked public declarations, without TLS material.
// State remains responsible for accepting the resulting runtime configuration.
type DecodedSource struct {
	Source
	NetworkID, OrderSeed       [32]byte
	Authorities                map[[32]byte]ed25519.PublicKey
	ClockObservation           time.Time
	Identities, LeafKeyDigests [2][32]byte
}

// DecodeSource checks the existing bounded Source grammar without file or network effects.
func DecodeSource(input []byte) (DecodedSource, error) {
	if len(input) > 32<<10 {
		return DecodedSource{}, errors.New("source plan exceeds its bound")
	}
	var plan Source
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return DecodedSource{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return DecodedSource{}, errors.New("source plan contains trailing JSON")
	}
	if plan.Schema != "ardents-source-plan-v1" || plan.LocalRoleStateRoot == "" || len(plan.Sources) != 2 {
		return DecodedSource{}, errors.New("source plan is not canonical or complete")
	}
	result := DecodedSource{Source: plan}
	if err := decodeFixedHex(plan.NetworkID, result.NetworkID[:]); err != nil {
		return DecodedSource{}, err
	}
	var err error
	result.Authorities, err = decodeAuthorities(plan.AuthorityPublic, 16)
	if err != nil {
		return DecodedSource{}, err
	}
	if plan.Threshold < 1 || plan.Threshold > len(result.Authorities) {
		return DecodedSource{}, errors.New("invalid declared threshold")
	}
	result.ClockObservation, err = time.Parse(time.RFC3339, plan.ClockObservedAt)
	if err != nil {
		return DecodedSource{}, err
	}
	if err := decodeFixedHex(plan.OrderSeed, result.OrderSeed[:]); err != nil {
		return DecodedSource{}, err
	}
	for index, member := range plan.Sources {
		if member.Family == "" {
			return DecodedSource{}, errors.New("source operator family is required")
		}
		if err := decodeFixedHex(member.Identity, result.Identities[index][:]); err != nil {
			return DecodedSource{}, err
		}
		if err := decodeFixedHex(member.LeafKeyDigest, result.LeafKeyDigests[index][:]); err != nil {
			return DecodedSource{}, err
		}
	}
	if plan.Sources[0].Family == plan.Sources[1].Family {
		return DecodedSource{}, errors.New("source operator families must be distinct")
	}
	return result, nil
}

func validateTextFields(plan Headless) error {
	if plan.TransitAcquisitionRoot != "" || plan.BytesEachDirection != 0 || plan.AlphaCorpusStateRoot != "" || plan.AlphaCorpusAuthority != "" || plan.AlphaCohort != "" {
		return errors.New("text runtime plan cannot select legacy acquisition or interfaces")
	}
	paths := []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.ApplicationSocket, plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath}
	permissions := []Permission{plan.ReaderPermission}
	if plan.Role == "reader" {
		if plan.PublicationRoot != "" || plan.ServiceInstanceRoot != "" || plan.AdministrationSocket != "" || plan.AdministrationPrincipal != "" || plan.PublisherPermission != (Permission{}) {
			return errors.New("reader runtime cannot select Publisher inputs")
		}
	} else {
		paths = append(paths, plan.PublicationRoot, plan.ServiceInstanceRoot, plan.AdministrationSocket, plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath)
		permissions = append(permissions, plan.PublisherPermission)
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		if !posixpath.IsAbs(path) || posixpath.Clean(path) != path || seen[path] {
			return errors.New("text runtime paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for index, permission := range permissions {
		maximum := uint64(4096)
		if index == 1 {
			maximum = 16384
		}
		total := uint64(permission.Maxima[0]) + uint64(permission.Maxima[1]) + uint64(permission.Maxima[2])
		if total == 0 || total > maximum {
			return errors.New("text runtime allocation unavailable")
		}
	}
	return nil
}
