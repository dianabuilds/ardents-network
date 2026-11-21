package endpoint

import (
	"encoding/json"
	"errors"
)

// These projections decode already bound Endpoint declarations for its runtime.
type PermissionPlan struct {
	RequestPath  string    `json:"request_path"`
	ResponsePath string    `json:"response_path"`
	Maxima       [3]uint32 `json:"maxima"`
}

type HeadlessPlan struct {
	Schema                  string         `json:"schema"`
	Role                    string         `json:"role"`
	NetworkStateRoot        string         `json:"network_state_root"`
	LocalRoleStateRoot      string         `json:"local_role_state_root"`
	TimeConfidenceFile      string         `json:"time_confidence_file"`
	TextTokenRoot           string         `json:"text_token_root"`
	EntryStateRoot          string         `json:"entry_state_root"`
	ApplicationSocket       string         `json:"application_socket"`
	AdministrationSocket    string         `json:"administration_socket"`
	PublicationRoot         string         `json:"publication_root"`
	ServiceInstanceRoot     string         `json:"service_instance_root"`
	NetworkID               string         `json:"network_id"`
	NetworkAuthorities      []string       `json:"network_authorities"`
	NetworkThreshold        int            `json:"network_threshold"`
	ClosedProfileAuthority  string         `json:"closed_profile_authority"`
	BrokerID                string         `json:"broker_id"`
	ConnectionPrincipal     string         `json:"connection_principal"`
	AdministrationPrincipal string         `json:"administration_principal"`
	ReaderPermission        PermissionPlan `json:"reader_permission"`
	PublisherPermission     PermissionPlan `json:"publisher_permission"`
}

type SourcePlan struct {
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

func Declarations(headlessRaw, sourceRaw []byte) (HeadlessPlan, SourcePlan, error) {
	var headless HeadlessPlan
	var source SourcePlan
	if len(headlessRaw) > 16<<10 || len(sourceRaw) > 32<<10 || json.Unmarshal(headlessRaw, &headless) != nil || json.Unmarshal(sourceRaw, &source) != nil || headless.Schema != "ardents-headless-runtime-v2" || source.Schema != "ardents-source-plan-v1" || len(source.Sources) != 2 {
		return headless, source, errors.New("installed bound declarations are unavailable")
	}
	return headless, source, nil
}
