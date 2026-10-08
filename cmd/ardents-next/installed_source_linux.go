//go:build linux

package main

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// These projections consume Installation's already bound declarations. They
// neither duplicate installation admission nor turn declarations into State.
type installedPermission struct {
	RequestPath  string    `json:"request_path"`
	ResponsePath string    `json:"response_path"`
	Maxima       [3]uint32 `json:"maxima"`
}

type installedHeadless struct {
	Schema                  string              `json:"schema"`
	Role                    string              `json:"role"`
	NetworkStateRoot        string              `json:"network_state_root"`
	LocalRoleStateRoot      string              `json:"local_role_state_root"`
	TimeConfidenceFile      string              `json:"time_confidence_file"`
	TextTokenRoot           string              `json:"text_token_root"`
	NetworkID               string              `json:"network_id"`
	NetworkAuthorities      []string            `json:"network_authorities"`
	NetworkThreshold        int                 `json:"network_threshold"`
	ClosedProfileAuthority  string              `json:"closed_profile_authority"`
	BrokerID                string              `json:"broker_id"`
	ConnectionPrincipal     string              `json:"connection_principal"`
	AdministrationPrincipal string              `json:"administration_principal"`
	ReaderPermission        installedPermission `json:"reader_permission"`
	PublisherPermission     installedPermission `json:"publisher_permission"`
}

type installedSource struct {
	Schema               string `json:"schema"`
	ClockObservedAt      string `json:"clock_observed_at"`
	OrderSeed            string `json:"order_seed"`
	MaterializationIndex uint32 `json:"materialization_index"`
	RefreshIntervalMS    uint32 `json:"refresh_interval_ms"`
	ClientCertificate    string `json:"client_certificate"`
	ClientKey            string `json:"client_key"`
	Sources              []struct {
		Address        string `json:"address"`
		ServerName     string `json:"server_name"`
		Identity       string `json:"identity"`
		Family         string `json:"family"`
		EndpointHandle string `json:"endpoint_handle"`
		RootCA         string `json:"root_ca"`
		LeafKeyDigest  string `json:"leaf_key_digest"`
	} `json:"sources"`
}

func installedDeclarations(headlessRaw, sourceRaw []byte) (installedHeadless, installedSource, error) {
	var headless installedHeadless
	var source installedSource
	if len(headlessRaw) > 16<<10 || len(sourceRaw) > 32<<10 || json.Unmarshal(headlessRaw, &headless) != nil || json.Unmarshal(sourceRaw, &source) != nil || headless.Schema != "ardents-headless-runtime-v2" || source.Schema != "ardents-source-plan-v1" || len(source.Sources) != 2 {
		return headless, source, errors.New("installed bound declarations are unavailable")
	}
	return headless, source, nil
}

func installedIdentity(encoded string) ([32]byte, error) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) || hex.EncodeToString(raw) != encoded {
		return value, errors.New("installed identity is unavailable")
	}
	copy(value[:], raw)
	return value, nil
}

func installedLocalConfig(plan installedHeadless) (execution.Config, error) {
	id, err := installedIdentity(plan.BrokerID)
	if err != nil {
		return execution.Config{}, err
	}
	reader, err := installedIdentity(plan.ConnectionPrincipal)
	if err != nil {
		return execution.Config{}, err
	}
	config := execution.Config{ID: id, Grants: []execution.Grant{{Principal: reader, Surface: execution.Connection}}}
	if plan.Role == "" {
		publisher, err := installedIdentity(plan.AdministrationPrincipal)
		if err != nil {
			return execution.Config{}, err
		}
		config.Grants = append(config.Grants, execution.Grant{Principal: publisher, Surface: execution.Administration})
	}
	return config, nil
}

func installedNetworkConfig(plan installedHeadless, declared installedSource, check func() error) (state.Config, error) {
	networkID, err := installedIdentity(plan.NetworkID)
	if err != nil {
		return state.Config{}, err
	}
	profileKey, err := installedIdentity(plan.ClosedProfileAuthority)
	if err != nil {
		return state.Config{}, err
	}
	authority := &networkAuthorityPlan{Root: plan.NetworkStateRoot, NetworkID: networkID, Threshold: plan.NetworkThreshold, ProfileAuthority: profileKey, ClockObservationFile: plan.TimeConfidenceFile}
	for _, encoded := range plan.NetworkAuthorities {
		key, err := installedIdentity(encoded)
		if err != nil {
			return state.Config{}, err
		}
		authority.Authorities = append(authority.Authorities, key)
	}
	config, err := networkStateConfig(authority)
	if err != nil {
		return state.Config{}, err
	}
	config.LocalRoleStateRoot = plan.LocalRoleStateRoot
	config.ClockObservation, err = time.Parse(time.RFC3339, declared.ClockObservedAt)
	if err != nil {
		return state.Config{}, err
	}
	config.Source.OrderSeed, err = installedIdentity(declared.OrderSeed)
	if err != nil {
		return state.Config{}, err
	}
	config.Source.MaterialIndex = declared.MaterializationIndex
	config.AutomaticRefreshInterval = time.Duration(declared.RefreshIntervalMS) * time.Millisecond
	read := func(path string, maximum int64) ([]byte, error) {
		if err := check(); err != nil {
			return nil, err
		}
		raw, err := readBounded(path, maximum)
		if checked := check(); checked != nil {
			clear(raw)
			return nil, errors.Join(err, checked)
		}
		return raw, err
	}
	certificate, err := read(declared.ClientCertificate, 64<<10)
	if err != nil {
		return state.Config{}, err
	}
	defer clear(certificate)
	key, err := read(declared.ClientKey, 16<<10)
	if err != nil {
		return state.Config{}, err
	}
	defer clear(key)
	config.Source.ClientCertificate, err = tls.X509KeyPair(certificate, key)
	if err != nil {
		return state.Config{}, err
	}
	for index, member := range declared.Sources {
		identity, err := installedIdentity(member.Identity)
		if err != nil {
			return state.Config{}, err
		}
		leaf, err := installedIdentity(member.LeafKeyDigest)
		if err != nil {
			return state.Config{}, err
		}
		root, err := read(member.RootCA, 64<<10)
		if err != nil {
			return state.Config{}, err
		}
		config.Source.Sources[index] = source.Source{Address: member.Address, ServerName: member.ServerName, Identity: identity, Family: member.Family, EndpointHandle: member.EndpointHandle, RootPEM: root, LeafKeyDigest: leaf}
	}
	return config, check()
}
