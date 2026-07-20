package config

type WorkloadsConfig struct {
	Executor            string         `json:"executor"`
	AllowedRegistries   []string       `json:"allowed_registries"`
	AllowedPolicyRefs   []string       `json:"allowed_policy_refs"`
	TrustedRuntime      string         `json:"trusted_runtime"`
	UntrustedRuntime    string         `json:"untrusted_runtime"`
	AllowedIngressHosts []string       `json:"allowed_ingress_hosts"`
	IngressBindAddress  string         `json:"ingress_bind_address"`
	IngressProxyImage   string         `json:"ingress_proxy_image"`
	Initial             []WorkloadSpec `json:"initial"`
}

type WorkloadSpec struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	Owner         string          `json:"owner"`
	Config        string          `json:"config"`
	Desired       string          `json:"desired"`
	Capabilities  []string        `json:"capabilities"`
	PolicyRef     string          `json:"policy_ref"`
	RestartPolicy string          `json:"restart_policy"`
	Services      []ServiceConfig `json:"services"`
}
