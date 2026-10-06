package installation

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	posixpath "path"
	"strings"
	"time"
)

// Request retains checked local declarations only. It grants no Release,
// Network, native ownership or runtime authority. Paths use the selected Linux
// installation grammar on every host; native admission is a later operation.
type Request struct{ declared *installationRequest }

// DecodeRequest checks one canonical bounded request before byte authentication.
// Initial input requires the independent pin; successor input forbids it.
func DecodeRequest(ctx context.Context, raw []byte, initial bool) (Request, error) {
	if ctx == nil {
		return Request{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	declared, err := decodeInstallationRequest(raw)
	if err != nil {
		return Request{}, errors.Join(ErrInput, err)
	}
	if initial && declared.ManifestSHA256 == "" || !initial && declared.ManifestSHA256 != "" {
		return Request{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	return Request{declared: &declared}, nil
}

func (r Request) BundleRoot() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.BundleRoot
}
func (r Request) ManifestSHA256() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.ManifestSHA256
}
func (r Request) ReleaseHistoryRoot() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.ReleaseFloorRoot
}
func (r Request) ReferenceTime() time.Time {
	if r.declared == nil {
		return time.Time{}
	}
	at, _ := time.Parse(time.RFC3339Nano, r.declared.ReferenceTime)
	return at
}

// permission declares the local request/response paths and finite resource maxima.
type permission struct {
	RequestPath  string    `json:"request_path"`
	ResponsePath string    `json:"response_path"`
	Maxima       [3]uint32 `json:"maxima"`
}

// headlessDeclaration contains the local inputs for one Target Link participant runtime
// and its Connection Interface, without selecting a Service Target or Route.
type headlessDeclaration struct {
	Role                string     `json:"role,omitempty"`
	TextTokenRoot       string     `json:"text_token_root,omitempty"`
	ReaderPermission    permission `json:"reader_permission,omitempty"`
	PublisherPermission permission `json:"publisher_permission,omitempty"`
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

// decodedHeadless projects checked declarations into pinned runtime identities.
type decodedHeadless struct {
	headlessDeclaration
	NetworkID, BrokerID, ConnectionPrincipal, AdministrationPrincipal [32]byte
	NetworkAuthorities                                                map[[32]byte]ed25519.PublicKey
	ClosedProfileAuthority                                            ed25519.PublicKey
}

// errHeadlessV1Retired identifies the unsupported historical runtime schema.
var errHeadlessV1Retired = errors.New("headless runtime plan v1 is retired")

// decodeHeadless checks a bounded v2 local plan without acquiring runtime resources.
func decodeHeadless(input []byte) (decodedHeadless, error) {
	if len(input) == 0 || len(input) > 16<<10 {
		return decodedHeadless{}, errors.New("headless runtime input exceeds its bound")
	}
	var raw headlessDeclaration
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return decodedHeadless{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return decodedHeadless{}, errors.New("operator input contains trailing JSON")
	}
	if raw.Schema == "ardents-headless-runtime-v1" {
		return decodedHeadless{}, errHeadlessV1Retired
	}
	if raw.Role != "" && raw.Role != "reader" {
		return decodedHeadless{}, errors.New("text runtime role is unavailable")
	}
	if raw.Schema != "ardents-headless-runtime-v2" || raw.NetworkStateRoot == "" || raw.EntryStateRoot == "" ||
		raw.ApplicationSocket == "" || !posixpath.IsAbs(raw.ApplicationSocket) ||
		raw.LocalRoleStateRoot == "" || raw.TimeConfidenceFile == "" || raw.NetworkProfile != "ardents-route-v3" || raw.BrokerID == "" ||
		raw.ConnectionPrincipal == "" || raw.Role == "" && (raw.AdministrationSocket == "" || !posixpath.IsAbs(raw.AdministrationSocket) || raw.ApplicationSocket == raw.AdministrationSocket || raw.PublicationRoot == "" || raw.AdministrationPrincipal == "") {
		return decodedHeadless{}, errors.New("headless runtime plan is incomplete")
	}
	if err := validateTextFields(raw); err != nil {
		return decodedHeadless{}, err
	}
	result := decodedHeadless{headlessDeclaration: raw}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{raw.NetworkID, result.NetworkID[:]}, {raw.BrokerID, result.BrokerID[:]}, {raw.ConnectionPrincipal, result.ConnectionPrincipal[:]}} {
		if err := decodeFixedHex(field.encoded, field.destination); err != nil {
			return decodedHeadless{}, err
		}
	}
	if raw.Role == "" {
		if err := decodeFixedHex(raw.AdministrationPrincipal, result.AdministrationPrincipal[:]); err != nil {
			return decodedHeadless{}, err
		}
	}
	authorities, err := decodeAuthorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return decodedHeadless{}, err
	}
	if raw.NetworkThreshold < 1 || raw.NetworkThreshold > len(authorities) {
		return decodedHeadless{}, errors.New("invalid declared threshold")
	}
	result.NetworkAuthorities = authorities
	authority := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if err := decodeFixedHex(raw.ClosedProfileAuthority, authority); err != nil {
		return decodedHeadless{}, fmt.Errorf("text State profile authority: %w", err)
	}
	if _, pinned := authorities[sha256.Sum256(authority)]; !pinned {
		return decodedHeadless{}, errors.New("text State profile authority is not pinned by State")
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

// sourceDeclaration declares an existing direct-Source configuration and credential paths.
// Decoding declarations never reads the referenced credentials.
type sourceDeclaration struct {
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
	Sources              []sourceMember `json:"sources"`
}

// sourceMember declares one pinned direct-Source endpoint.
type sourceMember struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

// decodedSource contains checked public declarations, without TLS material.
// State remains responsible for accepting the resulting runtime configuration.
type decodedSource struct {
	sourceDeclaration
	NetworkID, OrderSeed       [32]byte
	Authorities                map[[32]byte]ed25519.PublicKey
	ClockObservation           time.Time
	Identities, LeafKeyDigests [2][32]byte
}

// decodeSource checks the existing bounded Source grammar without file or network effects.
func decodeSource(input []byte) (decodedSource, error) {
	if len(input) > 32<<10 {
		return decodedSource{}, errors.New("source plan exceeds its bound")
	}
	var plan sourceDeclaration
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return decodedSource{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return decodedSource{}, errors.New("source plan contains trailing JSON")
	}
	if plan.Schema != "ardents-source-plan-v1" || plan.LocalRoleStateRoot == "" || len(plan.Sources) != 2 {
		return decodedSource{}, errors.New("source plan is not canonical or complete")
	}
	result := decodedSource{sourceDeclaration: plan}
	if err := decodeFixedHex(plan.NetworkID, result.NetworkID[:]); err != nil {
		return decodedSource{}, err
	}
	var err error
	result.Authorities, err = decodeAuthorities(plan.AuthorityPublic, 16)
	if err != nil {
		return decodedSource{}, err
	}
	if plan.Threshold < 1 || plan.Threshold > len(result.Authorities) {
		return decodedSource{}, errors.New("invalid declared threshold")
	}
	result.ClockObservation, err = time.Parse(time.RFC3339, plan.ClockObservedAt)
	if err != nil {
		return decodedSource{}, err
	}
	if err := decodeFixedHex(plan.OrderSeed, result.OrderSeed[:]); err != nil {
		return decodedSource{}, err
	}
	for index, member := range plan.Sources {
		if member.Family == "" {
			return decodedSource{}, errors.New("source operator family is required")
		}
		if err := decodeFixedHex(member.Identity, result.Identities[index][:]); err != nil {
			return decodedSource{}, err
		}
		if err := decodeFixedHex(member.LeafKeyDigest, result.LeafKeyDigests[index][:]); err != nil {
			return decodedSource{}, err
		}
	}
	if plan.Sources[0].Family == plan.Sources[1].Family {
		return decodedSource{}, errors.New("source operator families must be distinct")
	}
	return result, nil
}

func validateTextFields(plan headlessDeclaration) error {
	if plan.TransitAcquisitionRoot != "" || plan.BytesEachDirection != 0 || plan.AlphaCorpusStateRoot != "" || plan.AlphaCorpusAuthority != "" || plan.AlphaCohort != "" {
		return errors.New("text runtime plan cannot select legacy acquisition or interfaces")
	}
	paths := []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.ApplicationSocket, plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath}
	permissions := []permission{plan.ReaderPermission}
	if plan.Role == "reader" {
		if plan.PublicationRoot != "" || plan.ServiceInstanceRoot != "" || plan.AdministrationSocket != "" || plan.AdministrationPrincipal != "" || plan.PublisherPermission != (permission{}) {
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

// installationRequest is one root-declared installation input. It contains declarations
// and an independently supplied first-install pin, never permission responses,
// Authority keys or serialized Release authorizations.
type installationRequest struct {
	Schema           string              `json:"schema"`
	BundleRoot       string              `json:"bundle_root"`
	ManifestSHA256   string              `json:"manifest_sha256,omitempty"`
	InstallationRoot string              `json:"installation_root"`
	ReleaseFloorRoot string              `json:"release_floor_root"`
	ReferenceTime    string              `json:"reference_time"`
	Headless         headlessDeclaration `json:"headless"`
	Source           sourceDeclaration   `json:"source"`
}

func decodeInstallationRequest(raw []byte) (installationRequest, error) {
	var request installationRequest
	if err := decodeCanonical(raw, 64<<10, &request); err != nil {
		return installationRequest{}, err
	}
	if request.Schema != "ardents-endpoint-installation-request-v1" {
		return installationRequest{}, errors.New("installation request schema is invalid")
	}
	if request.ManifestSHA256 != "" && !canonicalDigest(request.ManifestSHA256) {
		return installationRequest{}, errors.New("installation pin is invalid")
	}
	at, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil || at.IsZero() || at.UTC().Format(time.RFC3339Nano) != request.ReferenceTime {
		return installationRequest{}, errors.New("installation reference time must be canonical UTC")
	}
	headlessRaw, err := json.Marshal(request.Headless)
	if err != nil {
		return installationRequest{}, err
	}
	headless, err := decodeHeadless(headlessRaw)
	if err != nil {
		return installationRequest{}, fmt.Errorf("installation headless declaration: %w", err)
	}
	sourceRaw, err := json.Marshal(request.Source)
	if err != nil {
		return installationRequest{}, err
	}
	source, err := decodeSource(sourceRaw)
	if err != nil {
		return installationRequest{}, fmt.Errorf("installation Source declaration: %w", err)
	}
	if headless.NetworkID != source.NetworkID || headless.NetworkThreshold != source.Threshold ||
		headless.LocalRoleStateRoot != source.LocalRoleStateRoot || headless.TimeConfidenceFile != source.ClockObservationFile ||
		source.RefreshIntervalMS == 0 || !sameSignerMap(headless.NetworkAuthorities, source.Authorities) {
		return installationRequest{}, errors.New("installation Source does not match its headless declaration")
	}
	// The selected generation renders this sole field to its bound Source file.
	// A caller cannot retain an external mutable Source plan as another owner.
	if request.Headless.NetworkSourcePlan != "" {
		return installationRequest{}, errors.New("installation Source must be inlined, not an external plan")
	}
	immutable := []string{request.BundleRoot, request.InstallationRoot, request.ReleaseFloorRoot}
	for index, path := range immutable {
		if !canonicalPath(path) || posixpath.Dir(path) == path {
			return installationRequest{}, errors.New("installation root path is invalid")
		}
		for _, other := range immutable[:index] {
			if pathsOverlap(path, other) {
				return installationRequest{}, errors.New("installation roots overlap")
			}
		}
	}
	roots := mutableRoots(request.Headless)
	for i, root := range roots {
		for _, prior := range roots[:i] {
			if pathsOverlap(root, prior) {
				return installationRequest{}, errors.New("declared mutable roots overlap")
			}
		}
	}
	paths := append(mutableRoots(request.Headless), request.Headless.ApplicationSocket, request.Headless.TimeConfidenceFile,
		request.Headless.ReaderPermission.RequestPath, request.Headless.ReaderPermission.ResponsePath,
		request.Source.ClientCertificate, request.Source.ClientKey)
	if request.Headless.Role == "" {
		paths = append(paths, request.Headless.AdministrationSocket, request.Headless.PublisherPermission.RequestPath, request.Headless.PublisherPermission.ResponsePath)
	}
	for _, member := range request.Source.Sources {
		paths = append(paths, member.RootCA)
	}
	for _, path := range paths {
		if !canonicalPath(path) {
			return installationRequest{}, errors.New("installation declaration path is invalid")
		}
		for _, root := range immutable {
			if pathsOverlap(path, root) {
				return installationRequest{}, errors.New("installation declarations overlap an immutable or Release root")
			}
		}
	}
	return request, nil
}

func decodeCanonical(raw []byte, maximum int, target any) error {
	if len(raw) == 0 || len(raw) > maximum {
		return errors.New("installation document exceeds its bound")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("installation document is invalid")
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("installation document is not canonical")
	}
	return nil
}

func canonicalDigest(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == 32 && hex.EncodeToString(digest) == value
}

func canonicalPath(path string) bool {
	return posixpath.IsAbs(path) && posixpath.Clean(path) == path && !strings.ContainsAny(path, "\x00\r\n")
}

func pathsOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func mutableRoots(plan headlessDeclaration) []string {
	roots := []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot}
	if plan.Role == "" {
		roots = append(roots, plan.PublicationRoot, plan.ServiceInstanceRoot)
	}
	return roots
}

func sameSignerMap(left, right map[[32]byte]ed25519.PublicKey) bool {
	if len(left) != len(right) {
		return false
	}
	for identity, key := range left {
		if !bytes.Equal(key, right[identity]) {
			return false
		}
	}
	return true
}
