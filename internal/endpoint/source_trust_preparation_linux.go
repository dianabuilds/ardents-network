//go:build linux

package endpoint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

var errSourceTrustPreparation = errors.New("source public trust preparation unavailable")

type sourcePublicFile struct {
	Path       string              `json:"path"`
	Commitment sourcePreparationID `json:"sha256"`
}

type sourcePreparationMember struct {
	Identity          sourcePreparationID `json:"identity"`
	Family            string              `json:"family"`
	Handle            string              `json:"endpoint_handle"`
	Address           string              `json:"address"`
	ServerName        string              `json:"server_name"`
	Certificate       sourcePublicFile    `json:"certificate"`
	Root              sourcePublicFile    `json:"root"`
	Pin               sourcePreparationID `json:"leaf_key_digest"`
	PrivateKey        string              `json:"private_key"`
	StateRoot         string              `json:"state_root"`
	ParticipationRoot string              `json:"participation_root"`
}

type sourcePreparationPlan struct {
	Schema            string                    `json:"schema"`
	Reference         string                    `json:"reference_time"`
	ClockObservedAt   string                    `json:"clock_observed_at"`
	Client            HeadlessPlan              `json:"client"`
	ClientCertificate sourcePublicFile          `json:"client_certificate"`
	ClientRoot        sourcePublicFile          `json:"client_root"`
	ClientPin         sourcePreparationID       `json:"client_leaf_key_digest"`
	ClientKey         string                    `json:"client_private_key"`
	OrderSeed         sourcePreparationID       `json:"order_seed"`
	MaterialIndex     uint32                    `json:"materialization_index"`
	RefreshMS         uint32                    `json:"refresh_interval_ms"`
	Sources           []sourcePreparationMember `json:"sources"`
}

func decodeSourcePreparation(raw []byte) (sourcePreparationPlan, time.Time, error) {
	var p sourcePreparationPlan
	bad := errSourceTrustPreparation
	var fields map[string]json.RawMessage
	if len(raw) > 64<<10 || artifactio.DecodeDeclaration(raw, &fields, "schema", "reference_time", "clock_observed_at", "client", "client_certificate", "client_root", "client_leaf_key_digest", "client_private_key", "order_seed", "materialization_index", "refresh_interval_ms", "sources") != nil ||
		json.Unmarshal(raw, &p) != nil || p.Schema != "ardents-source-trust-preparation-v1" || len(p.Sources) != 2 {
		return p, time.Time{}, bad
	}

	var client map[string]json.RawMessage
	if artifactio.DecodeDeclaration(fields["client"], &client, "schema", "role", "network_state_root", "local_role_state_root", "time_confidence_file", "text_token_root", "entry_state_root", "application_socket", "administration_socket", "publication_root", "service_instance_root", "network_id", "network_authorities", "network_threshold", "closed_profile_authority", "broker_id", "connection_principal", "administration_principal", "reader_permission", "publisher_permission") != nil {
		return p, time.Time{}, bad
	}
	for _, field := range []string{"reader_permission", "publisher_permission"} {
		var permission map[string]json.RawMessage
		var maxima []uint32
		if artifactio.DecodeDeclaration(client[field], &permission, "request_path", "response_path", "maxima") != nil ||
			json.Unmarshal(permission["maxima"], &maxima) != nil || len(maxima) != 3 {
			return p, time.Time{}, bad
		}
	}

	var members []json.RawMessage
	if json.Unmarshal(fields["sources"], &members) != nil {
		return p, time.Time{}, bad
	}
	for _, member := range members {
		var entry map[string]json.RawMessage
		if artifactio.DecodeDeclaration(member, &entry, "identity", "family", "endpoint_handle", "address", "server_name", "certificate", "root", "leaf_key_digest", "private_key", "state_root", "participation_root") != nil ||
			!validSourcePublicFile(entry["certificate"]) || !validSourcePublicFile(entry["root"]) {
			return p, time.Time{}, bad
		}
	}
	if !validSourcePublicFile(fields["client_certificate"]) || !validSourcePublicFile(fields["client_root"]) {
		return p, time.Time{}, bad
	}
	reference, err := sourcePreparationTime(p.Reference)
	if _, e := sourcePreparationTime(p.ClockObservedAt); e != nil {
		return p, time.Time{}, bad
	}
	if err != nil || p.MaterialIndex >= 64 || p.RefreshMS < 100 || p.RefreshMS > 60000 || p.OrderSeed == (sourcePreparationID{}) || p.Client.Schema != "ardents-headless-runtime-v2" {
		return p, time.Time{}, bad
	}
	return p, reference, nil
}

type sourcePreparationID [32]byte

func (id *sourcePreparationID) UnmarshalJSON(raw []byte) error {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return errSourceTrustPreparation
	}
	key, err := installedIdentity(text)
	if err != nil {
		return errSourceTrustPreparation
	}
	*id = key
	return nil
}

func validSourcePublicFile(raw []byte) bool {
	var file map[string]json.RawMessage
	return artifactio.DecodeDeclaration(raw, &file, "path", "sha256") == nil
}

func sourcePreparationTime(text string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339, text)
	if err != nil || value.Location() != time.UTC || value.Nanosecond() != 0 || value.Format(time.RFC3339) != text {
		return time.Time{}, errSourceTrustPreparation
	}
	return value, nil
}

// PrepareSourceTrust writes the existing public Source and Endpoint declarations
// after a bounded offline observation. It opens only the declared public inputs.
func PrepareSourceTrust(ctx context.Context, path, destination string, observe func(string, *os.File)) (receipt map[string]string, err error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, errSourceTrustPreparation
	}
	plan, err := artifactio.OpenProtectedInput(ctx, path, 64<<10, false)
	if err != nil {
		return nil, err
	}
	inputs := []*artifactio.ProtectedInput{plan}
	defer func() {
		for i := len(inputs) - 1; i >= 0; i-- {
			err = errors.Join(err, inputs[i].Close())
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			receipt = nil
		}
	}()
	if err = artifactio.RequireExt4(plan.Descriptor()); err != nil {
		return nil, err
	}
	p, reference, err := decodeSourcePreparation(plan.Bytes())
	if err != nil {
		return nil, err
	}
	config, err := p.Client.Network()
	if err != nil || config.Threshold < 1 || config.Threshold > len(config.Authorities) {
		return nil, errSourceTrustPreparation
	}
	authority := servingdeclaration.ServingNetwork{
		Root: config.Root, NetworkID: config.NetworkID, Threshold: config.Threshold,
		ProfileAuthority: [32]byte(config.ClosedProfileAuthority), ClockObservationFile: config.ClockObservationFile,
	}
	for _, encoded := range p.Client.NetworkAuthorities {
		key, err := installedIdentity(encoded)
		if err != nil {
			return nil, err
		}
		authority.Authorities = append(authority.Authorities, key)
	}
	// All runtime roots are opaque declarations. Check separation without opening
	// them. The existing pure configuration projection performs no file I/O;
	// no State owner is opened offline.
	roots := []string{p.Client.NetworkStateRoot, p.Client.LocalRoleStateRoot}
	privatePaths := []string{p.ClientKey}
	for _, member := range p.Sources {
		roots = append(roots, member.StateRoot, member.ParticipationRoot)
		privatePaths = append(privatePaths, member.PrivateKey)
	}
	if !independentSourcePreparationRoots(append(roots, destination)...) {
		return nil, errSourceTrustPreparation
	}
	for i, private := range privatePaths {
		if !absoluteSourcePreparationPath(private) || private == destination || strings.HasPrefix(private, destination+string(filepath.Separator)) {
			return nil, errSourceTrustPreparation
		}
		for _, other := range privatePaths[:i] {
			if private == other {
				return nil, errSourceTrustPreparation
			}
		}
	}
	check := func() error {
		if os.Geteuid() != 0 {
			return errSourceTrustPreparation
		}
		for _, input := range inputs {
			if e := input.Check(ctx); e != nil {
				return e
			}
		}
		now := time.Now()
		if now.Before(reference.Add(-time.Minute)) || now.After(reference.Add(5*time.Minute)) {
			return errSourceTrustPreparation
		}
		return ctx.Err()
	}
	var output []artifactio.Content
	read := func(declared sourcePublicFile, name string) ([]byte, error) {
		if !absoluteSourcePreparationPath(declared.Path) {
			return nil, errSourceTrustPreparation
		}
		for _, private := range privatePaths {
			if declared.Path == private {
				return nil, errSourceTrustPreparation
			}
		}
		input, e := artifactio.OpenProtectedInput(ctx, declared.Path, 64<<10, false)
		if e != nil {
			return nil, e
		}
		inputs = append(inputs, input)
		if e = artifactio.RequireExt4(input.Descriptor()); e != nil {
			return nil, e
		}
		if sha256.Sum256(input.Bytes()) != [32]byte(declared.Commitment) {
			return nil, errSourceTrustPreparation
		}
		output = append(output, artifactio.Content{Name: name, Data: input.Bytes()})
		return input.Bytes(), check()
	}
	if err = check(); err != nil {
		return nil, err
	}
	var trust source.PublicTrust
	trust.ClientCertificatePEM, err = read(p.ClientCertificate, "client-certificate.pem")
	if err != nil {
		return nil, err
	}
	trust.ClientRootPEM, err = read(p.ClientRoot, "client-root.pem")
	if err != nil {
		return nil, err
	}
	trust.ClientKeyDigest = p.ClientPin
	// Reuse the existing installed Source declaration, without introducing a
	// second runtime schema or granting Installation startup admission.
	declared := SourcePlan{Schema: "ardents-source-plan-v1", ClockObservedAt: p.ClockObservedAt, OrderSeed: hex.EncodeToString(p.OrderSeed[:]), MaterializationIndex: p.MaterialIndex, RefreshIntervalMS: p.RefreshMS, ClientCertificate: filepath.Join(destination, "client-certificate.pem"), ClientKey: p.ClientKey}
	appendJSON := func(name string, value any) error {
		raw, e := canonicalPlanJSON(value)
		if e != nil {
			return e
		}
		if len(raw) > 8<<20 {
			return errSourceTrustPreparation
		}
		output = append(output, artifactio.Content{Name: name, Data: raw})
		return nil
	}
	declared.Sources = slices.Grow(declared.Sources, len(p.Sources))[:len(p.Sources)]
	for i, member := range p.Sources {
		name := fmt.Sprintf("source-%d", i+1)
		chain, e := read(member.Certificate, name+"-certificate.pem")
		if e != nil {
			return nil, e
		}
		root, e := read(member.Root, name+"-root.pem")
		if e != nil {
			return nil, e
		}
		trust.Sources[i] = source.PublicSource{Source: source.Source{Identity: member.Identity, Family: member.Family, EndpointHandle: member.Handle, Address: member.Address, ServerName: member.ServerName, RootPEM: root, LeafKeyDigest: member.Pin}, CertificatePEM: chain}
		entry := &declared.Sources[i]
		entry.Address = member.Address
		entry.ServerName = member.ServerName
		entry.Identity = hex.EncodeToString(member.Identity[:])
		entry.Family = member.Family
		entry.EndpointHandle = member.Handle
		entry.RootCA = filepath.Join(destination, name+"-root.pem")
		entry.LeafKeyDigest = hex.EncodeToString(member.Pin[:])
		network := authority
		network.Root = member.StateRoot
		serving := servingdeclaration.ServingDeclaration{Network: &network, ProcessProfile: processresource.SourceProfile, ParticipationRoot: member.ParticipationRoot, Address: member.Address, Certificate: filepath.Join(destination, name+"-certificate.pem"), PrivateKey: member.PrivateKey, ClientRoot: filepath.Join(destination, "client-root.pem"), ClientPins: [][32]byte{p.ClientPin}}
		if member.Address == "" {
			return nil, errSourceTrustPreparation
		}
		if err = appendJSON(name+"-serve.json", serving); err != nil {
			return nil, err
		}
	}
	if err = source.ObservePublicTrust(trust, config.Authorities, reference); err != nil {
		return nil, err
	}
	if err = appendJSON("client-source.json", declared); err != nil {
		return nil, err
	}
	if err = appendJSON("client-headless.json", p.Client); err != nil {
		return nil, err
	}
	if observe != nil {
		observe("inputs-read", plan.Descriptor())
	}
	if err = artifactio.PublishInventory(ctx, destination, output, false, check, observe); err != nil {
		return nil, err
	}
	receipt = make(map[string]string, len(output))
	for _, file := range output {
		digest := sha256.Sum256(file.Data)
		receipt[file.Name] = hex.EncodeToString(digest[:])
	}
	return receipt, nil
}

// These declarations name independent roots without opening them.
func independentSourcePreparationRoots(roots ...string) bool {
	for i, root := range roots {
		if !absoluteSourcePreparationPath(root) {
			return false
		}
		for _, previous := range roots[:i] {
			if root == previous || strings.HasPrefix(root, previous+"/") || strings.HasPrefix(previous, root+"/") {
				return false
			}
		}
	}
	return true
}

func absoluteSourcePreparationPath(path string) bool {
	return pathgrammar.Strict(path)
}
