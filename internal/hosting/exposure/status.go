package exposure

import hostingservice "ardents/internal/hosting/service"

type PublishedStatus struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner"`
	Mode           string   `json:"mode"`
	Published      bool     `json:"published"`
	Endpoints      []string `json:"endpoints,omitempty"`
	ProbeEndpoints []string `json:"probe_endpoints,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type WorkloadView struct {
	WorkloadID        string
	DeclaredServices  []hostingservice.Spec
	PublishedServices []PublishedStatus
}

type Denial struct {
	ID  string
	Err error
}

type PolicyFunc func(hostingservice.Spec) error

func EffectivePublishedServices(items []PublishedStatus, allow PolicyFunc) []PublishedStatus {
	if len(items) == 0 {
		return nil
	}
	out := make([]PublishedStatus, 0, len(items))
	for _, item := range items {
		if item.Published && allow != nil {
			if err := allow(hostingservice.Spec{
				ID:             item.ID,
				Type:           item.Type,
				Mode:           item.Mode,
				Endpoints:      append([]string(nil), item.Endpoints...),
				ProbeEndpoints: append([]string(nil), item.ProbeEndpoints...),
			}); err != nil {
				item.Published = false
				item.Reason = err.Error()
			}
		}
		item.Endpoints = append([]string(nil), item.Endpoints...)
		item.ProbeEndpoints = append([]string(nil), item.ProbeEndpoints...)
		out = append(out, item)
	}
	return out
}
