package hosting

import "time"

type EndpointSnapshot struct {
	Kind      string `json:"kind,omitempty"`
	Address   string `json:"address,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Port      int    `json:"port,omitempty"`
	Exposure  string `json:"exposure,omitempty"`
	Reachable bool   `json:"reachable,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ServiceSnapshot struct {
	ID                 string             `json:"id,omitempty"`
	Type               string             `json:"type,omitempty"`
	Owner              string             `json:"owner,omitempty"`
	WorkloadID         string             `json:"workload_id,omitempty"`
	Visibility         string             `json:"visibility,omitempty"`
	DesiredPublication string             `json:"desired_publication,omitempty"`
	RuntimeBacking     string             `json:"runtime_backing,omitempty"`
	PolicyRef          string             `json:"policy_ref,omitempty"`
	Readiness          string             `json:"readiness,omitempty"`
	Ready              bool               `json:"ready,omitempty"`
	ExposureEligible   bool               `json:"exposure_eligible,omitempty"`
	Generation         int64              `json:"generation,omitempty"`
	LastProbeAt        time.Time          `json:"last_probe_at"`
	Endpoints          []EndpointSnapshot `json:"endpoints,omitempty"`
}

type PublicationSnapshot struct {
	State                  string     `json:"state,omitempty"`
	Reason                 string     `json:"reason,omitempty"`
	Published              bool       `json:"published,omitempty"`
	PublishedAt            time.Time  `json:"published_at"`
	ExpiresAt              time.Time  `json:"expires_at"`
	WithdrawnAt            *time.Time `json:"withdrawn_at,omitempty"`
	OperatorActionRequired bool       `json:"operator_action_required,omitempty"`
}

type ServiceStatusSnapshot struct {
	ServiceID              string              `json:"service_id,omitempty"`
	State                  string              `json:"state,omitempty"`
	Reason                 string              `json:"reason,omitempty"`
	Published              bool                `json:"published,omitempty"`
	RuntimeBacking         string              `json:"runtime_backing,omitempty"`
	Ready                  bool                `json:"ready,omitempty"`
	ExposureEligible       bool                `json:"exposure_eligible,omitempty"`
	Generation             int64               `json:"generation,omitempty"`
	LastProbeAt            time.Time           `json:"last_probe_at"`
	Publication            PublicationSnapshot `json:"publication"`
	LastTransitionAt       time.Time           `json:"last_transition_at"`
	OperatorActionRequired bool                `json:"operator_action_required,omitempty"`
}
