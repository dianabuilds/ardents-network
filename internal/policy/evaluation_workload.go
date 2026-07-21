package policy

import (
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/registry"
	"fmt"
)

type WorkloadConfig struct {
	MaxWorkloads       int
	AllowedPolicyRefs  []string
	DeniedCapabilities []string
}

func CheckWorkload(cfg WorkloadConfig, spec domainworkload.Spec, existing []execution.Status) Result {
	if cfg.MaxWorkloads > 0 {
		count := len(existing)
		if !containsWorkload(existing, spec.ID) && count >= cfg.MaxWorkloads {
			return Deny("policy_admission_denied", fmt.Sprintf("workload limit %d reached", cfg.MaxWorkloads))
		}
	}
	if spec.PolicyRef != "" && !ContainsNormalized(cfg.AllowedPolicyRefs, spec.PolicyRef) {
		return Deny("policy_admission_denied", "policy reference is not allowed")
	}
	for _, capability := range spec.Capabilities {
		if ContainsNormalized(cfg.DeniedCapabilities, capability) {
			return Deny("policy_admission_denied", "workload capability is denied by policy")
		}
	}
	return Allow()
}

func containsWorkload(items []execution.Status, id string) bool {
	for _, item := range items {
		if item.Spec.ID == id {
			return true
		}
	}
	return false
}
