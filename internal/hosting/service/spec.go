package service

type Spec struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner,omitempty"`
	Mode           string   `json:"mode"`
	Endpoints      []string `json:"endpoints,omitempty"`
	ProbeEndpoints []string `json:"probe_endpoints,omitempty"`
}
