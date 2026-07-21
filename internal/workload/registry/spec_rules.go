package registry

import "fmt"

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
	return nil
}

func NormalizeSpec(spec Spec) Spec {
	if spec.Services == nil {
		spec.Services = []ServiceSpec{}
	}
	if spec.Capabilities == nil {
		spec.Capabilities = []string{}
	}
	spec.Desired = NormalizeDesired(spec.Desired)
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
	return spec
}
