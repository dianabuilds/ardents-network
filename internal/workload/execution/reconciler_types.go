package execution

import (
	"ardents/internal/workload/registry"
)

const (
	DefaultRestartPolicy = registry.DefaultRestartPolicy
	DefaultRestartBudget = 2
)

type AdmissionFunc func(registry.Spec, []Status) error

type persistentState struct {
	Items map[string]Status `json:"items"`
}
