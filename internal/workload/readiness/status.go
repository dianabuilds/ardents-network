package readiness

import "time"

type Status struct {
	ID               string
	Type             string
	Owner            string
	Mode             string
	Published        bool
	State            string
	Ready            bool
	ExposureEligible bool
	Generation       int64
	LastProbeAt      time.Time
	ReadinessReason  string
	Reason           string
	Source           string
	Endpoints        []string
	EndpointStatuses []EndpointStatus
}
