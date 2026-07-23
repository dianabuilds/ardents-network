package policy

import (
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/registry"
	"fmt"
	"slices"
)

type WorkloadConfig struct {
	MaxWorkloads               int
	AllowedPolicyRefs          []string
	DeniedWorkloadRequirements []domainworkload.WorkloadRequirement
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
	if err := domainworkload.ValidateWorkloadRequirements(cfg.DeniedWorkloadRequirements); err != nil {
		return Deny("policy_admission_denied", "workload requirement policy is invalid")
	}
	if err := domainworkload.ValidateSpec(spec); err != nil {
		return Deny("policy_admission_denied", "workload specification is invalid")
	}
	for _, requirement := range spec.Requirements {
		if slices.Contains(cfg.DeniedWorkloadRequirements, requirement) {
			return Deny("policy_admission_denied", "workload requirement is denied by policy")
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
