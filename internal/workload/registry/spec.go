package registry

const DefaultRestartPolicy = "on-failure"

type ServiceSpec struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner,omitempty"`
	Mode           string   `json:"mode"`
	Endpoints      []string `json:"endpoints,omitempty"`
	ProbeEndpoints []string `json:"probe_endpoints,omitempty"`
}

type Spec struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Owner         string        `json:"owner"`
	Config        string        `json:"config,omitempty"`
	Desired       string        `json:"desired"`
	Services      []ServiceSpec `json:"services,omitempty"`
	Capabilities  []string      `json:"capabilities,omitempty"`
	PolicyRef     string        `json:"policy_ref,omitempty"`
	RestartPolicy string        `json:"restart_policy,omitempty"`
}
