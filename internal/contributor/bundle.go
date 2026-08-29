package contributor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var bundleFiles = []string{
	"ardents-node", "node.json", "rendezvous-cert.pem", "rendezvous-key.pem", "rendezvous-identity.pem",
	"source-client-cert.pem", "source-client-key.pem", "source-a-root.pem", "source-b-root.pem", "clock.observation",
}

type bundleManifest struct {
	Schema       string            `json:"schema"`
	Profile      string            `json:"profile"`
	DeploymentID string            `json:"deployment_id"`
	Generation   uint64            `json:"generation"`
	Files        map[string]string `json:"files"`
}

type verifiedBundle struct {
	manifest       bundleManifest
	manifestDigest string
	files          map[string][]byte
}

type profileNodePlan struct {
	Schema                  string          `json:"schema"`
	StateRoot               string          `json:"state_root"`
	LocalRoleStateRoot      string          `json:"local_role_state_root"`
	NetworkID               string          `json:"network_id"`
	AuthorityPublic         []string        `json:"authority_public"`
	Threshold               int             `json:"threshold"`
	ServerCertificate       string          `json:"server_certificate"`
	ServerKey               string          `json:"server_key"`
	MaterializationIndex    uint32          `json:"materialization_index"`
	ClockObservationFile    string          `json:"clock_observation_file"`
	OrderSeed               string          `json:"order_seed"`
	SourceClientCertificate string          `json:"source_client_certificate"`
	SourceClientKey         string          `json:"source_client_key"`
	Sources                 []profileSource `json:"sources"`
	NodeID                  string          `json:"node_id"`
	IdentityKey             string          `json:"identity_key"`
	NodeResourceProfile     string          `json:"node_resource_profile"`
	DiagnosticDirectory     string          `json:"diagnostic_directory"`
	Rendezvous              profileDuty     `json:"rendezvous"`
}

type profileSource struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

type profileDuty struct {
	HandshakeLimit     uint16 `json:"handshake_limit"`
	WaitingLimit       uint16 `json:"waiting_limit"`
	PairLimit          uint16 `json:"pair_limit"`
	PairByteLimit      uint64 `json:"pair_byte_limit"`
	AdmissionTimeoutMS uint32 `json:"admission_timeout_ms"`
	DrainTimeoutMS     uint32 `json:"drain_timeout_ms"`
}

func openBundle(directory, pin string) (verifiedBundle, error) {
	if !filepath.IsAbs(directory) || !fixedHex(pin, sha256.Size) {
		return verifiedBundle{}, errors.New("contributor bundle path or manifest pin is invalid")
	}
	manifestRaw, err := readRegular(filepath.Join(directory, "manifest.json"), 64<<10)
	if err != nil {
		return verifiedBundle{}, err
	}
	digest := sha256.Sum256(manifestRaw)
	if hex.EncodeToString(digest[:]) != pin {
		return verifiedBundle{}, errors.New("contributor bundle manifest does not match its independent pin")
	}
	var manifest bundleManifest
	if err := decodeStrict(manifestRaw, &manifest); err != nil {
		return verifiedBundle{}, fmt.Errorf("decode Contributor manifest: %w", err)
	}
	if manifest.Schema != "ardents-contributor-bundle-v1" || manifest.Profile != profileName || manifest.Generation == 0 ||
		!fixedHex(manifest.DeploymentID, 32) || len(manifest.Files) != len(bundleFiles) {
		return verifiedBundle{}, errors.New("contributor bundle manifest is not canonical")
	}
	result := verifiedBundle{manifest: manifest, manifestDigest: pin, files: make(map[string][]byte, len(bundleFiles))}
	for _, name := range bundleFiles {
		want, ok := manifest.Files[name]
		if !ok || !fixedHex(want, sha256.Size) {
			return verifiedBundle{}, errors.New("contributor bundle file inventory is incomplete")
		}
		maximum := int64(64 << 10)
		if name == "ardents-node" {
			maximum = 128 << 20
		}
		raw, readErr := readRegular(filepath.Join(directory, name), maximum)
		if readErr != nil {
			return verifiedBundle{}, readErr
		}
		actual := sha256.Sum256(raw)
		if hex.EncodeToString(actual[:]) != want {
			return verifiedBundle{}, fmt.Errorf("contributor bundle file %s does not match its digest", name)
		}
		result.files[name] = raw
	}
	if err := validateProfilePlan(result.files["node.json"]); err != nil {
		return verifiedBundle{}, err
	}
	return result, nil
}

func validateProfilePlan(raw []byte) error {
	var plan profileNodePlan
	if err := decodeStrict(raw, &plan); err != nil {
		return fmt.Errorf("decode Contributor Node plan: %w", err)
	}
	if plan.Schema != "ardents-node-plan-v1" || plan.StateRoot != "/var/lib/private/ardents-contributor/network" ||
		plan.LocalRoleStateRoot != "/var/lib/private/ardents-contributor/role" || !fixedHex(plan.NetworkID, 32) ||
		plan.Threshold < 1 || plan.Threshold > len(plan.AuthorityPublic) || len(plan.AuthorityPublic) > 16 ||
		plan.ServerCertificate != installedPath("rendezvous-cert.pem") || plan.ServerKey != installedPath("rendezvous-key.pem") ||
		plan.ClockObservationFile != installedPath("clock.observation") || !fixedHex(plan.OrderSeed, 32) ||
		plan.SourceClientCertificate != installedPath("source-client-cert.pem") || plan.SourceClientKey != installedPath("source-client-key.pem") ||
		!fixedHex(plan.NodeID, 32) || plan.IdentityKey != installedPath("rendezvous-identity.pem") ||
		plan.NodeResourceProfile != profileName || plan.DiagnosticDirectory != "/var/lib/private/ardents-contributor/diagnostics" {
		return errors.New("contributor Node plan does not match the dedicated-host profile")
	}
	for _, authority := range plan.AuthorityPublic {
		if !fixedHex(authority, 32) {
			return errors.New("contributor Node plan authority is invalid")
		}
	}
	if len(plan.Sources) != 2 {
		return errors.New("contributor Node plan requires exactly two authenticated Sources")
	}
	for index, source := range plan.Sources {
		host, port, err := net.SplitHostPort(source.Address)
		root := installedPath("source-a-root.pem")
		if index == 1 {
			root = installedPath("source-b-root.pem")
		}
		if err != nil || net.ParseIP(host) == nil || port == "" || source.ServerName == "" || !fixedHex(source.Identity, 32) ||
			source.Family == "" || source.EndpointHandle == "" || source.RootCA != root || !fixedHex(source.LeafKeyDigest, 32) {
			return errors.New("contributor Node plan Source is invalid")
		}
	}
	if plan.Rendezvous != (profileDuty{HandshakeLimit: 4, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 16 << 20,
		AdmissionTimeoutMS: 5000, DrainTimeoutMS: 5000}) {
		return errors.New("contributor Rendezvous reservations do not match the functional-alpha profile")
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("input contains trailing JSON")
	}
	return nil
}

func fixedHex(value string, size int) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == size
}

func readRegular(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, errors.New("contributor bundle input is absent, non-regular, or outside its bound")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if int64(len(raw)) > maximum {
		return nil, errors.New("contributor bundle input exceeds its bound")
	}
	return raw, errors.Join(readErr, closeErr)
}
