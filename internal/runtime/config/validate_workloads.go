package config

import (
	"fmt"
	"net/url"
	"strings"

	networkapi "ardents/internal/network/api"
	"ardents/internal/workload/execution"
)

func validateWorkloads(doc Document) error {
	switch doc.Workloads.Executor {
	case "docker":
	case "trusted-process":
		if doc.Node.Profile != string(networkapi.NodeProfileLocalDevelopment) {
			return fmt.Errorf("workloads.executor=trusted-process requires local_development node profile")
		}
	default:
		return fmt.Errorf("workloads.executor: unsupported mode %q", doc.Workloads.Executor)
	}
	if err := validateServiceList("services", doc.Services, false, ""); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	allowedPolicyRefs := normalizedStrings(effectiveAllowedPolicyRefs(doc))
	deniedCapabilities := normalizedStrings(doc.Policy.DeniedCapabilities)
	for index, workload := range doc.Workloads.Initial {
		path := fmt.Sprintf("workloads.initial[%d]", index)
		if err := validateWorkload(path, workload); err != nil {
			return err
		}
		if doc.Workloads.Executor == "docker" && workload.Config != "" {
			if err := execution.ValidateContainerConfig(workload.Config); err != nil {
				return fmt.Errorf("%s.config: %w", path, err)
			}
		}
		if err := validateWorkloadPolicy(path, workload, allowedPolicyRefs, deniedCapabilities); err != nil {
			return err
		}
		if _, duplicate := seen[workload.ID]; duplicate {
			return fmt.Errorf("%s.id is duplicated", path)
		}
		seen[workload.ID] = struct{}{}
		if err := validateServiceList(path+".services", workload.Services, true, workload.ID); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkloadPolicy(path string, workload WorkloadSpec, allowed, denied map[string]struct{}) error {
	if workload.PolicyRef != "" {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(workload.PolicyRef))]; !ok {
			return fmt.Errorf("%s.policy_ref is not allowed", path)
		}
	}
	for _, capability := range workload.Capabilities {
		if _, ok := denied[strings.ToLower(strings.TrimSpace(capability))]; ok {
			return fmt.Errorf("%s.capabilities contains a policy-denied capability", path)
		}
	}
	return nil
}

func effectiveAllowedPolicyRefs(doc Document) []string {
	if len(doc.Policy.AllowedPolicyRefs) > 0 {
		return doc.Policy.AllowedPolicyRefs
	}
	return doc.Workloads.AllowedPolicyRefs
}

func normalizedStrings(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func validateWorkload(path string, workload WorkloadSpec) error {
	if strings.TrimSpace(workload.ID) == "" || strings.TrimSpace(workload.Owner) == "" {
		return fmt.Errorf("%s id and owner are required", path)
	}
	switch workload.Kind {
	case "service", "worker", "app", "adapter":
	default:
		return fmt.Errorf("%s.kind is unsupported", path)
	}
	switch workload.Desired {
	case "present", "running", "stopped", "disabled", "removed":
	default:
		return fmt.Errorf("%s.desired is unsupported", path)
	}
	if workload.Desired == "running" && strings.TrimSpace(workload.Config) == "" {
		return fmt.Errorf("%s.config is required when desired=running", path)
	}
	if workload.RestartPolicy != "" && workload.RestartPolicy != "on-failure" && workload.RestartPolicy != "never" {
		return fmt.Errorf("%s.restart_policy is unsupported", path)
	}
	return nil
}

func validateServiceList(path string, services []ServiceConfig, paired bool, expectedOwner string) error {
	seen := map[string]struct{}{}
	for index, service := range services {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if strings.TrimSpace(service.ID) == "" || strings.TrimSpace(service.Type) == "" {
			return fmt.Errorf("%s id and type are required", itemPath)
		}
		if expectedOwner == "" && strings.TrimSpace(service.Owner) == "" {
			return fmt.Errorf("%s.owner is required", itemPath)
		}
		if expectedOwner != "" && service.Owner != "" && service.Owner != expectedOwner {
			return fmt.Errorf("%s.owner must match its workload", itemPath)
		}
		if _, duplicate := seen[service.ID]; duplicate {
			return fmt.Errorf("%s.id is duplicated", itemPath)
		}
		seen[service.ID] = struct{}{}
		if service.Mode != "NetworkPublished" && service.Mode != "LocalOnly" {
			return fmt.Errorf("%s.mode is unsupported", itemPath)
		}
		if len(service.Endpoints) == 0 {
			return fmt.Errorf("%s requires at least one endpoint", itemPath)
		}
		if paired && len(service.Endpoints) != len(service.ProbeEndpoints) {
			return fmt.Errorf("%s requires paired endpoint and probe sets", itemPath)
		}
		if !paired && len(service.ProbeEndpoints) > 0 && len(service.Endpoints) != len(service.ProbeEndpoints) {
			return fmt.Errorf("%s endpoint and probe sets must have equal length", itemPath)
		}
		for _, endpoint := range append(append([]string(nil), service.Endpoints...), service.ProbeEndpoints...) {
			parsed, err := url.Parse(endpoint)
			if err != nil || parsed.Scheme == "" {
				return fmt.Errorf("%s contains an invalid endpoint", itemPath)
			}
		}
	}
	return nil
}
