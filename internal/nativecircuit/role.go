package nativecircuit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

const nativeRoleSchema = "carrier-lab-native-role/v1"

var nativeRunID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type roleHop struct {
	Address           string `json:"address"`
	CertificateSHA256 string `json:"certificate_sha256"`
}

type roleConfig struct {
	SchemaVersion       string    `json:"schema_version"`
	RunID               string    `json:"run_id"`
	Role                string    `json:"role"`
	ListenAddress       string    `json:"listen_address,omitempty"`
	CertificatePath     string    `json:"certificate_path,omitempty"`
	PrivateKeyPath      string    `json:"private_key_path,omitempty"`
	AllowedNext         []string  `json:"allowed_next,omitempty"`
	ExpectedConnections int       `json:"expected_connections,omitempty"`
	StartPath           string    `json:"start_path,omitempty"`
	Profile             string    `json:"profile,omitempty"`
	Rendezvous          string    `json:"rendezvous,omitempty"`
	SlotHex             string    `json:"slot_hex,omitempty"`
	IntroductionPath    []roleHop `json:"introduction_path,omitempty"`
	DataPath            []roleHop `json:"data_path,omitempty"`
	HPKEPublicPath      string    `json:"hpke_public_path,omitempty"`
	HPKEPrivatePath     string    `json:"hpke_private_path,omitempty"`
	TargetRootPath      string    `json:"target_root_path,omitempty"`
	ExpectedLeafSHA256  string    `json:"expected_leaf_sha256,omitempty"`
	EndpointCertificate string    `json:"endpoint_certificate_path,omitempty"`
	EndpointPrivateKey  string    `json:"endpoint_private_key_path,omitempty"`
	PayloadSeed         string    `json:"payload_seed,omitempty"`
	PayloadBytes        int       `json:"payload_bytes,omitempty"`
	StreamDirection     string    `json:"stream_direction,omitempty"`
	StreamSeed          string    `json:"stream_seed,omitempty"`
	StreamDuration      int       `json:"stream_duration_seconds,omitempty"`
	Fault               string    `json:"fault,omitempty"`
	DirectAddress       string    `json:"direct_address,omitempty"`
	AttachedSocket      string    `json:"attached_socket,omitempty"`
}

// RunRole executes one fixed native C-5/C2 role from a role-local data-only
// configuration and writes bounded role evidence.
func RunRole(ctx context.Context, configPath, evidenceDir string) error {
	config, err := readRoleConfig(configPath)
	if err != nil {
		return err
	}
	if err := validateRoleConfig(config); err != nil {
		return err
	}
	info, err := os.Stat(evidenceDir)
	if err != nil || !info.IsDir() {
		return errors.New("native role evidence directory must already exist")
	}
	return runConfiguredRole(ctx, config, evidenceDir)
}

func readRoleConfig(path string) (roleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return roleConfig{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config roleConfig
	if err := decoder.Decode(&config); err != nil || decoder.More() {
		return roleConfig{}, errors.New("native role configuration has invalid encoding")
	}
	if config.SchemaVersion != nativeRoleSchema || !nativeRunID.MatchString(config.RunID) {
		return roleConfig{}, errors.New("native role schema or run identity is invalid")
	}
	return config, nil
}

func validateRoleConfig(config roleConfig) error {
	if config.Profile == directProfile {
		return validateDirectRoleConfig(config)
	}
	if isRelayRole(config.Role) || config.Role == "rendezvous" || config.Role == "introduction-node" {
		if config.ListenAddress == "" || config.CertificatePath == "" || config.PrivateKeyPath == "" || config.ExpectedConnections < 1 {
			return errors.New("native Node role configuration is incomplete")
		}
		if hasEndpointKnowledge(config) {
			return fmt.Errorf("%s configuration contains endpoint-only knowledge", config.Role)
		}
		if isRelayRole(config.Role) && len(config.AllowedNext) == 0 {
			return errors.New("native relay has no allowed adjacent role")
		}
		if !isRelayRole(config.Role) && len(config.AllowedNext) != 0 {
			return errors.New("terminal native role received relay configuration")
		}
		return nil
	}
	if config.Role != "user" && config.Role != "service" {
		return errors.New("native role is not part of the fixed C-5/C2 topology")
	}
	return validateEndpointRoleConfig(config)
}

func isRelayRole(role string) bool {
	switch role {
	case "user-entry", "user-interior", "service-interior", "data-service-entry", "introduction-forwarder", "introduction-interior", "introduction-entry":
		return true
	default:
		return false
	}
}

func hasEndpointKnowledge(config roleConfig) bool {
	return config.StartPath != "" || config.Profile != "" || config.Rendezvous != "" || config.SlotHex != "" ||
		len(config.IntroductionPath) != 0 || len(config.DataPath) != 0 || config.HPKEPublicPath != "" ||
		config.HPKEPrivatePath != "" || config.TargetRootPath != "" || config.ExpectedLeafSHA256 != "" ||
		config.EndpointCertificate != "" || config.EndpointPrivateKey != "" || config.PayloadSeed != "" || config.PayloadBytes != 0 ||
		config.StreamDirection != "" || config.StreamSeed != "" || config.StreamDuration != 0 || config.DirectAddress != "" || config.AttachedSocket != "" || config.Fault != ""

}

func validateDirectRoleConfig(config roleConfig) error {
	if config.Role != "user" && config.Role != "service" || config.DirectAddress == "" || config.SlotHex == "" {
		return errors.New("native Direct role configuration is incomplete")
	}
	streaming := config.StreamDirection != "" || config.StreamSeed != "" || config.StreamDuration != 0
	streamValid := validateStreamSpec(streamSpec{Direction: config.StreamDirection, Seed: config.StreamSeed, Duration: time.Duration(config.StreamDuration) * time.Second}, false) == nil
	if streaming && !streamValid {
		return errors.New("native Direct stream configuration is invalid")
	}
	if config.Role == "user" {
		payloadValid := config.PayloadSeed != "" && config.PayloadBytes >= 1 && config.PayloadBytes <= maximumQueueBytes
		if config.TargetRootPath == "" || config.ExpectedLeafSHA256 == "" || !streaming && !payloadValid || config.EndpointCertificate != "" || config.EndpointPrivateKey != "" {
			return errors.New("native Direct User knowledge is invalid")
		}
		return nil
	}
	if config.EndpointCertificate == "" || config.EndpointPrivateKey == "" || config.TargetRootPath != "" || config.ExpectedLeafSHA256 != "" || config.PayloadSeed != "" || config.PayloadBytes != 0 {
		return errors.New("native Direct Service knowledge is invalid")
	}
	return nil
}

func validateEndpointRoleConfig(config roleConfig) error {
	if config.ListenAddress != "" || config.CertificatePath != "" || config.PrivateKeyPath != "" || len(config.AllowedNext) != 0 || config.ExpectedConnections != 0 {
		return errors.New("native endpoint received ordinary Node configuration")
	}
	if config.StartPath == "" || config.Profile != candidateProfile && config.Profile != c3Profile || config.Rendezvous == "" || config.SlotHex == "" {
		return errors.New("native endpoint route configuration is incomplete")
	}
	if config.Role == "user" {
		dataLength, introductionLength := 3, 4
		if config.Profile == c3Profile {
			dataLength, introductionLength = 2, 2
		}
		streaming := config.StreamDirection != "" || config.StreamSeed != "" || config.StreamDuration != 0
		streamValid := validateStreamSpec(streamSpec{Direction: config.StreamDirection, Seed: config.StreamSeed, Duration: time.Duration(config.StreamDuration) * time.Second}, false) == nil
		payloadValid := config.PayloadSeed != "" && config.PayloadBytes >= 1 && config.PayloadBytes <= maximumQueueBytes
		attached := validAttachedSocket(config.AttachedSocket)
		if len(config.DataPath) != dataLength || len(config.IntroductionPath) != introductionLength || config.HPKEPublicPath == "" || config.TargetRootPath == "" || config.ExpectedLeafSHA256 == "" || streaming && !streamValid || !streaming && !attached && !payloadValid || streaming && (config.PayloadSeed != "" || config.PayloadBytes != 0 || attached) || attached && (config.PayloadSeed != "" || config.PayloadBytes != 0) || config.HPKEPrivatePath != "" || config.EndpointPrivateKey != "" || config.Fault != "" && config.Fault != "rendezvous-process" {
			return errors.New("native User configuration violates the fixed knowledge boundary")
		}
		return nil
	}
	streaming := config.StreamDirection != "" || config.StreamSeed != "" || config.StreamDuration != 0
	streamValid := validateStreamSpec(streamSpec{Direction: config.StreamDirection, Seed: config.StreamSeed, Duration: time.Duration(config.StreamDuration) * time.Second}, false) == nil
	dataLength, introductionLength := 3, 3
	if config.Profile == c3Profile {
		dataLength, introductionLength = 2, 2
	}
	attached := validAttachedSocket(config.AttachedSocket)
	if len(config.DataPath) != dataLength || len(config.IntroductionPath) != introductionLength || config.HPKEPrivatePath == "" || config.EndpointCertificate == "" || config.EndpointPrivateKey == "" || config.HPKEPublicPath != "" || config.TargetRootPath != "" || config.ExpectedLeafSHA256 != "" || config.PayloadSeed != "" || config.PayloadBytes != 0 || streaming && !streamValid || streaming && attached || !streaming && config.AttachedSocket != "" && !attached || config.Fault != "" {
		return errors.New("native Service configuration violates the fixed knowledge boundary")
	}
	return nil
}

func validAttachedSocket(path string) bool {
	return path == "/attached/user/app.sock" || path == "/attached/service/app.sock"
}
