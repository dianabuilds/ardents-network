package daemon

import (
	"testing"

	workloadregistry "ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

func TestConfigMappingsCloneSlices(t *testing.T) {
	serviceCfg := ServiceConfig{
		ID:        "svc-1",
		Type:      "http",
		Owner:     "node",
		Mode:      "hosted",
		Endpoints: []string{"tcp://127.0.0.1:9000"},
	}
	policyCfg := PolicyConfig{
		AllowedPolicyRefs:          []string{"policy-a"},
		DeniedWorkloadRequirements: []workloadregistry.WorkloadRequirement{"exec"},
		DeniedServiceTypes:         []string{"db"},
		DeniedRouteSchemes:         []string{"udp"},
	}

	services := runtimeServiceConfigs([]ServiceConfig{serviceCfg})
	workloads := runtimeWorkloadSpecs([]WorkloadConfig{{
		ID:       "work-1",
		Kind:     "service",
		Owner:    "node",
		Desired:  "running",
		Services: []ServiceConfig{serviceCfg},
	}})
	policy := runtimePolicyConfig(policyCfg)

	services[0].Endpoints[0] = "mutated"
	workloads[0].Services[0].Endpoints[0] = "mutated"
	policy.AllowedPolicyRefs[0] = "mutated"
	policy.DeniedWorkloadRequirements[0] = "mutated"
	policy.DeniedServiceTypes[0] = "mutated"
	policy.DeniedRouteSchemes[0] = "mutated"

	require.Equal(t, "tcp://127.0.0.1:9000", serviceCfg.Endpoints[0])
	require.Equal(t, "policy-a", policyCfg.AllowedPolicyRefs[0])
	require.Equal(t, workloadregistry.WorkloadRequirement("exec"), policyCfg.DeniedWorkloadRequirements[0])
	require.Equal(t, "db", policyCfg.DeniedServiceTypes[0])
	require.Equal(t, "udp", policyCfg.DeniedRouteSchemes[0])
}
