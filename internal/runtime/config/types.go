package config

type Document struct {
	APIVersion    string              `json:"api_version"`
	Node          NodeConfig          `json:"node"`
	API           APIConfig           `json:"api"`
	Network       NetworkConfig       `json:"network"`
	Privacy       PrivacyConfig       `json:"privacy"`
	Workloads     WorkloadsConfig     `json:"workloads"`
	Services      []ServiceConfig     `json:"services"`
	Data          DataConfig          `json:"data"`
	Policy        PolicyConfig        `json:"policy"`
	Logging       LoggingConfig       `json:"logging"`
	Observability ObservabilityConfig `json:"observability"`
	Diagnostics   DiagnosticsConfig   `json:"diagnostics"`
}

type NodeConfig struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
	DataDir string `json:"data_dir"`
}

type APIConfig struct {
	ListenAddress       string   `json:"listen_address"`
	TokenFile           string   `json:"token_file"`
	OperatorSubject     string   `json:"operator_subject"`
	Capabilities        []string `json:"capabilities"`
	CredentialExpiresAt string   `json:"credential_expires_at"`
}

type PrivacyConfig struct {
	Required               bool                 `json:"required"`
	CapabilityStore        string               `json:"capability_store"`
	CapabilityStoreKeyFile string               `json:"capability_store_key_file"`
	ReplayKeyFile          string               `json:"replay_key_file"`
	Subject                string               `json:"subject"`
	TrustedIssuers         map[string]string    `json:"trusted_issuers"`
	Discovery              PrivacyChannelConfig `json:"discovery"`
	Data                   PrivacyChannelConfig `json:"data"`
}

type PrivacyChannelConfig struct {
	Reference  string `json:"reference"`
	ReplayPath string `json:"replay_path"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type ObservabilityConfig struct {
	ListenAddress string `json:"listen_address"`
	TokenFile     string `json:"token_file"`
}

type DiagnosticsConfig struct {
	MaxEvents   int    `json:"max_events"`
	DetailLevel string `json:"detail_level"`
}

type ServiceConfig struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner"`
	Mode           string   `json:"mode"`
	Endpoints      []string `json:"endpoints"`
	ProbeEndpoints []string `json:"probe_endpoints"`
}
