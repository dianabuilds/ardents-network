package registry

import (
	"fmt"
	"slices"
)

func ValidateSpec(spec Spec) error {
	if spec.ID == "" {
		return fmt.Errorf("missing workload id")
	}
	if spec.Kind == "" {
		return fmt.Errorf("missing workload kind")
	}
	switch spec.Kind {
	case "service", "worker", "app", "adapter":
	default:
		return fmt.Errorf("unsupported workload kind %s", spec.Kind)
	}
	if spec.Config == "invalid" {
		return fmt.Errorf("invalid config reference")
	}
	if err := ValidateWorkloadRequirements(spec.Requirements); err != nil {
		return err
	}
	return nil
}

func NormalizeSpec(spec Spec) Spec {
	if spec.Services == nil {
		spec.Services = []ServiceSpec{}
	}
	if spec.Requirements == nil {
		spec.Requirements = []WorkloadRequirement{}
	} else {
		spec.Requirements = append([]WorkloadRequirement(nil), spec.Requirements...)
		slices.Sort(spec.Requirements)
	}
	spec.Desired = NormalizeDesired(spec.Desired)
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
	return spec
}
