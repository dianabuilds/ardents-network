package namedsite

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const roleConfigSchema = "gatec-role-config/v1"

type authorityRoleConfig struct {
	Schema             string             `json:"schema"`
	RunID              string             `json:"run_id"`
	NetworkID          string             `json:"network_id"`
	Target             string             `json:"target"`
	NamePublic         []byte             `json:"name_public"`
	NamePrivate        []byte             `json:"name_private"`
	ServicePublic      []byte             `json:"service_public"`
	ServicePrivate     []byte             `json:"service_private"`
	InstancePublic     []byte             `json:"instance_public"`
	InstancePrivate    []byte             `json:"instance_private"`
	InstanceGeneration uint64             `json:"instance_generation"`
	Credential         instanceCredential `json:"credential"`
	AdminRequests      int                `json:"admin_requests"`
}

type clientRoleConfig struct {
	Schema             string `json:"schema"`
	RunID              string `json:"run_id"`
	NetworkID          string `json:"network_id"`
	Target             string `json:"target"`
	NamePublic         []byte `json:"name_public"`
	ServicePublic      []byte `json:"service_public"`
	InstanceGeneration uint64 `json:"instance_generation"`
}

type administrationRoleConfig struct {
	Schema               string              `json:"schema"`
	SupersededCredential *instanceCredential `json:"superseded_credential,omitempty"`
}

type authorityRequest struct {
	Operation    string              `json:"operation"`
	Type         string              `json:"type,omitempty"`
	Nonce        string              `json:"nonce,omitempty"`
	DeadlineUnix int64               `json:"deadline_unix,omitempty"`
	Credential   *instanceCredential `json:"credential,omitempty"`
}

type authorityResponse struct {
	Target             string              `json:"target,omitempty"`
	InstanceGeneration uint64              `json:"instance_generation,omitempty"`
	Credential         *instanceCredential `json:"credential,omitempty"`
	Record             []byte              `json:"record,omitempty"`
	Accepted           *bool               `json:"accepted,omitempty"`
}

func authorityConfig(fixture *authorityFixture) authorityRoleConfig {
	return authorityRoleConfig{
		Schema: roleConfigSchema, RunID: fixture.runID, NetworkID: fixture.networkID, Target: fixture.target,
		NamePublic: fixture.namePublic, NamePrivate: fixture.namePrivate, ServicePublic: fixture.servicePublic,
		ServicePrivate: fixture.servicePrivate, InstancePublic: fixture.instancePublic, InstancePrivate: fixture.instancePrivate,
		InstanceGeneration: fixture.instanceGeneration, Credential: fixture.credential, AdminRequests: 1,
	}
}

func publicClientConfig(fixture *authorityFixture) clientRoleConfig {
	return clientRoleConfig{Schema: roleConfigSchema, RunID: fixture.runID, NetworkID: fixture.networkID, Target: fixture.target, NamePublic: fixture.namePublic, ServicePublic: fixture.servicePublic, InstanceGeneration: fixture.instanceGeneration}
}

func (config authorityRoleConfig) fixture() (*authorityFixture, error) {
	if config.Schema != roleConfigSchema || config.RunID == "" || config.NetworkID == "" || config.Target == "" || config.InstanceGeneration == 0 || len(config.NamePrivate) != ed25519.PrivateKeySize || len(config.ServicePrivate) != ed25519.PrivateKeySize || len(config.InstancePrivate) != ed25519.PrivateKeySize {
		return nil, errors.New("authority role configuration is incomplete")
	}
	return &authorityFixture{
		runID: config.RunID, networkID: config.NetworkID, target: config.Target,
		namePublic: config.NamePublic, namePrivate: config.NamePrivate, servicePublic: config.ServicePublic, servicePrivate: config.ServicePrivate,
		instancePublic: config.InstancePublic, instancePrivate: config.InstancePrivate, instanceGeneration: config.InstanceGeneration, credential: config.Credential,
	}, nil
}

func readStrictRoleConfig(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 64*1024 {
		return errors.New("role configuration is missing or unbounded")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("role configuration has trailing data")
	}
	return nil
}
