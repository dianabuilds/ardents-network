package catalog_test

import (
	"testing"

	"ardents/internal/cli/catalog"

	"github.com/stretchr/testify/require"
)

func TestClosedCatalogueContainsExactlyCurrentLeafCommands(t *testing.T) {
	specs := catalog.Commands()

	require.Len(t, specs, 70)
	require.NoError(t, catalog.Validate(specs, nil))

	ids := make(map[string]struct{}, len(specs))
	paths := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		require.NotEmpty(t, spec.ID)
		require.NotEmpty(t, spec.Path)
		require.NotEmpty(t, spec.EvidenceOwner)
		require.NotEmpty(t, spec.Usage)
		require.Contains(t, catalog.KnownOutputShapes(), spec.Output)
		_, duplicateID := ids[spec.ID]
		require.Falsef(t, duplicateID, "duplicate command ID %q", spec.ID)
		ids[spec.ID] = struct{}{}
		path := catalog.PathString(spec.Path)
		_, duplicatePath := paths[path]
		require.Falsef(t, duplicatePath, "duplicate command path %q", path)
		paths[path] = struct{}{}
	}
}

func TestValidatorFailsClosedForInvalidCatalogue(t *testing.T) {
	valid := catalog.Commands()
	require.NotEmpty(t, valid)
	validRule := catalog.ProcedureRule{
		Access:       valid[0].Access,
		Action:       valid[0].Action,
		ResourceKind: valid[0].ResourceKind,
		Mutating:     valid[0].Mutating,
	}
	resolver := func(procedure string) (catalog.ProcedureRule, bool) {
		return validRule, procedure == valid[0].Procedure
	}

	tests := map[string]func([]catalog.CommandSpec) []catalog.CommandSpec{
		"empty catalogue": func([]catalog.CommandSpec) []catalog.CommandSpec { return nil },
		"duplicate command ID": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[1].ID = specs[0].ID
			return specs
		},
		"duplicate command path": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[1].Path = append([]string(nil), specs[0].Path...)
			return specs
		},
		"unknown output shape": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Output = catalog.OutputShape("unknown")
			return specs[:1]
		},
		"missing evidence owner": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].EvidenceOwner = ""
			return specs[:1]
		},
		"protected command without procedure": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Procedure = ""
			return specs[:1]
		},
		"protected command without action": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Action = ""
			return specs[:1]
		},
		"protected command without resource": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].ResourceKind = ""
			return specs[:1]
		},
		"protected command without SSH": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].SSH = false
			return specs[:1]
		},
		"offline command with RPC": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[50].Procedure = valid[0].Procedure
			return specs[50:51]
		},
		"offline command with action": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[50].Action = "node.status"
			return specs[50:51]
		},
		"offline command with SSH": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[50].SSH = true
			return specs[50:51]
		},
		"unknown procedure": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Procedure = "/ardents.v1.NodeService/Unknown"
			return specs[:1]
		},
		"sibling action": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Action = "node.stop"
			return specs[:1]
		},
		"resource mismatch": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].ResourceKind = "configuration"
			return specs[:1]
		},
		"mutation mismatch": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Mutating = !specs[0].Mutating
			return specs[:1]
		},
		"missing help entry": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Usage = ""
			return specs[:1]
		},
		"missing help summary": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Summary = ""
			return specs[:1]
		},
		"json command without output contract": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[0].Output = ""
			return specs[:1]
		},
		"unknown watch output": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			specs[6].WatchOutput = catalog.OutputShape("unknown")
			return specs[6:7]
		},
		"human-only command classified online": func(specs []catalog.CommandSpec) []catalog.CommandSpec {
			last := len(specs) - 3
			specs[last].Access = catalog.AccessProtected
			specs[last].Procedure = valid[0].Procedure
			specs[last].Action = valid[0].Action
			specs[last].ResourceKind = valid[0].ResourceKind
			return specs[last : last+1]
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			specs := mutate(catalog.Commands())
			err := catalog.Validate(specs, resolver)
			require.Errorf(t, err, "invalid case unexpectedly passed: %s", name)
		})
	}
}

func TestCatalogueContainsResearchPacketLeafPaths(t *testing.T) {
	expected := []string{
		"node start", "node stop", "node status", "node runtime", "node features", "node events",
		"network status", "network discovery", "network presence", "network peers", "network routes",
		"network resolve record", "network resolve service", "network records list", "network records import",
		"workload list", "workload get", "workload register", "workload start", "workload stop",
		"workload restart", "workload services", "workload service", "workload publication",
		"data inventory", "data objects list", "data objects get", "data objects publish",
		"data blobs list", "data blobs get", "data blobs publish", "data blobs fetch", "data blobs sources",
		"data blobs retain", "data blobs pin", "data blobs drop",
		"data manifests list", "data manifests get", "data manifests publish",
		"data transfers list", "data transfers get",
		"diagnostics snapshot", "diagnostics health", "diagnostics pending", "diagnostics explain", "diagnostics events",
		"config show", "config reload",
		"authority create", "authority inspect",
		"identity principal create", "identity principal import", "identity principal show",
		"identity device create", "identity device show", "identity device revoke",
		"identity enroll", "identity grant list", "identity grant issue", "identity grant revoke",
		"identity delegation issue", "identity delegation revoke", "identity delegation import-revocation",
		"identity application-ticket issue", "identity login", "identity status", "identity logout",
		"shell", "tui", "version",
	}

	actual := make([]string, 0, len(catalog.Commands()))
	for _, spec := range catalog.Commands() {
		actual = append(actual, catalog.PathString(spec.Path))
	}
	require.ElementsMatch(t, expected, actual)
}

func TestReachabilityValidatorRejectsMissingAndUnregisteredPaths(t *testing.T) {
	specs := catalog.Commands()[:2]
	reachable := []string{catalog.PathString(specs[0].Path), catalog.PathString(specs[1].Path)}
	require.NoError(t, catalog.ValidateReachability(specs, reachable))
	require.Error(t, catalog.ValidateReachability(specs, reachable[:1]), "registered but unreachable command passed")
	require.Error(t, catalog.ValidateReachability(specs, append(reachable, "node undocumented")), "uncatalogued parser command passed")
	require.Error(t, catalog.ValidateReachability(specs, append(reachable, reachable[0])), "duplicate parser command passed")
}

func TestOutputFamiliesRemainCompatibleWithCurrentRenderers(t *testing.T) {
	watchPaths := map[string]bool{
		"network status":      true,
		"diagnostics health":  true,
		"data transfers list": true,
		"data transfers get":  true,
	}
	for _, spec := range catalog.Commands() {
		path := catalog.PathString(spec.Path)
		switch {
		case path == "shell" || path == "tui":
			require.Equal(t, catalog.OutputHumanOnly, spec.Output, path)
		case path == "node events":
			require.Equal(t, catalog.OutputJSONLines, spec.Output, path)
		case spec.Path[0] == "identity" || path == "version":
			require.Equal(t, catalog.OutputCLIJSON, spec.Output, path)
		default:
			require.Equal(t, catalog.OutputProtoJSON, spec.Output, path)
		}
		if watchPaths[path] {
			require.Equal(t, catalog.OutputJSONLines, spec.WatchOutput, path)
		} else {
			require.Empty(t, spec.WatchOutput, path)
		}
	}
}

func TestEvidenceOwnershipHandoffCoversEveryLeaf(t *testing.T) {
	counts := map[string]int{}
	for _, spec := range catalog.Commands() {
		counts[spec.EvidenceOwner]++
	}
	require.Equal(t, map[string]int{
		"CGA-01": 2,
		"OCS-01": 12,
		"OCS-02": 20,
		"OCS-03": 9,
		"OCS-04": 17,
		"OCS-05": 10,
	}, counts)
}

func TestCatalogueLookupsReturnDefensiveCopies(t *testing.T) {
	matched, ok := catalog.Match([]string{"node", "start"})
	require.True(t, ok)
	matched.Path[0] = "mutated"
	exact, ok := catalog.Exact([]string{"node", "start"})
	require.True(t, ok)
	require.Equal(t, "node", exact.Path[0])
	require.NoError(t, catalog.ClosedError())
}
