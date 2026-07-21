package publication

import (
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/registry"
)

type Denial struct {
	ID  string
	Err error
}

type PolicyFunc func(domainworkload.ServiceSpec) error

func EffectivePublishedServices(items []execution.PublishedServiceStatus, allow PolicyFunc) []execution.PublishedServiceStatus {
	if len(items) == 0 {
		return nil
	}
	out := make([]execution.PublishedServiceStatus, 0, len(items))
	for _, item := range items {
		if item.Published && allow != nil {
			if err := allow(domainworkload.ServiceSpec{
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
