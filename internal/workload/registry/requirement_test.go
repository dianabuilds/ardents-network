package registry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkloadRequirementAcceptsOnlyCanonicalBoundedNames(t *testing.T) {
	for _, value := range []string{"gpu", "network.read", "net-bind", "memory_2g"} {
		requirement, err := ParseWorkloadRequirement(value)
		require.NoError(t, err)
		require.Equal(t, value, requirement.String())
	}
	for _, value := range []string{"", " GPU ", "Node:Read", ".gpu", "gpu.", "gpu..admin", "gpu/admin", strings.Repeat("x", MaxWorkloadRequirementBytes+1)} {
		_, err := ParseWorkloadRequirement(value)
		require.Error(t, err, value)
	}
}

func TestWorkloadRequirementJSONRejectsMalformedValuesAtomically(t *testing.T) {
	original, err := ParseWorkloadRequirement("gpu")
	require.NoError(t, err)
	target := original
	require.Error(t, json.Unmarshal([]byte(`" GPU "`), &target))
	require.Equal(t, original, target)
}
