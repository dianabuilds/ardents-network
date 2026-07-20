package projection

import (
	"testing"
	"time"

	discoveryapi "ardents/internal/discovery/api"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"

	"github.com/stretchr/testify/require"
)

func TestServiceSpecsSkipsIncompleteItems(t *testing.T) {
	items := ServiceSpecs([]ServiceConfig{
		{ID: "svc-a", Type: "http", Owner: "node", Mode: "public", Endpoints: []string{"http://a"}},
		{ID: "", Type: "http"},
	})
	if len(items) != 1 {
		t.Fatalf("items = %#v, want single valid service spec", items)
	}
	if items[0].ID != "svc-a" || items[0].Endpoints[0] != "http://a" {
		t.Fatalf("item = %#v, want cloned service spec", items[0])
	}
}

func TestWorkloadSpecsMapsNestedServices(t *testing.T) {
	items := WorkloadSpecs([]WorkloadConfig{{
		ID:      "work-a",
		Kind:    "service",
		Owner:   "node",
		Desired: "running",
		Services: []ServiceConfig{{
			ID: "svc-a", Type: "http", Mode: "public", Endpoints: []string{"http://a"},
		}},
	}})
	if len(items) != 1 || len(items[0].Services) != 1 {
		t.Fatalf("items = %#v, want workload with nested service", items)
	}
}

func TestPolicyAndDataServiceConfig(t *testing.T) {
	policy := PolicyServiceConfig(PolicyConfig{
		MaxWorkloads:         3,
		DeniedCapabilities:   []string{"net.admin"},
		MaxLocalRetentionTTL: int64(time.Hour),
	})
	if policy.MaxWorkloads != 3 || len(policy.DeniedCapabilities) != 1 {
		t.Fatalf("policy = %#v, want mapped policy config", policy)
	}

	data := DataServiceConfig(DataConfig{
		DefaultLocalRetentionTTL: int64(time.Minute),
		MaxRelayRetentionBytes:   42,
	})
	if data.DefaultLocalRetentionTTL != time.Minute || data.MaxRelayRetentionBytes != 42 {
		t.Fatalf("data = %#v, want mapped data config", data)
	}
}

func TestSplitTopicSplitsPrefixAndSuffix(t *testing.T) {
	domain, eventType := SplitTopic("workload.updated")
	require.Equal(t, "workload", domain)
	require.Equal(t, "updated", eventType)
}

func TestCloneMapCopiesValues(t *testing.T) {
	in := map[string]any{"a": "b"}
	out := CloneMap(in)
	out["a"] = "changed"
	require.Equal(t, "b", in["a"])
}

func TestWorkloadSnapshotClonesSlices(t *testing.T) {
	in := observedstate.Status{
		Spec: domainworkload.Spec{
			ID:           "work.echo",
			Config:       `{"env":{"API_TOKEN":"must-not-leak"}}`,
			Capabilities: []string{"net"},
			Services: []domainworkload.ServiceSpec{{
				ID:        "svc.echo",
				Endpoints: []string{"tcp://127.0.0.1:9000"},
			}},
		},
		PublishedServices: []observedstate.PublishedServiceStatus{{
			ID:        "svc.echo",
			Endpoints: []string{"tcp://127.0.0.1:9000"},
		}},
	}

	out := WorkloadSnapshot(in)
	require.Empty(t, out.Spec.Config)
	out.Spec.Capabilities[0] = "mutated"
	out.Spec.Services[0].Endpoints[0] = "mutated"
	out.PublishedServices[0].Endpoints[0] = "mutated"

	require.Equal(t, "net", in.Spec.Capabilities[0])
	require.Equal(t, "tcp://127.0.0.1:9000", in.Spec.Services[0].Endpoints[0])
	require.Equal(t, "tcp://127.0.0.1:9000", in.PublishedServices[0].Endpoints[0])
}

func TestDiscoveryRecordClonesEndpoints(t *testing.T) {
	in := discoveryapi.DiscoveryRecord{
		ID:        "rec-1",
		Endpoints: []string{"tcp://127.0.0.1:9000"},
	}

	out := DiscoveryRecord(in)
	out.Endpoints[0] = "mutated"

	require.Equal(t, "tcp://127.0.0.1:9000", in.Endpoints[0])
}
