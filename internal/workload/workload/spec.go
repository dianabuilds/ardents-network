package workload

import hostingservice "ardents/internal/hosting/service"

const DefaultRestartPolicy = "on-failure"

type ServiceSpec = hostingservice.Spec

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
