//go:build linux || darwin

package provisioning

import (
	"errors"
	"testing"
)

func TestNodeProvisioningRoleKeepsPublicReceiptAndPrivateFailureSeparate(t *testing.T) {
	cause := errors.New("private path failure")
	for _, tc := range []struct {
		role      nodeProvisioningRole
		operation string
		refusal   string
	}{
		{forwardingProvisioningRole, "forwarding.prepare-node-key", "forwarding artifact preparation refused"},
		{joinProvisioningRole, "join.prepare-node-key", "JOIN artifact preparation refused"},
		{descriptorProvisioningRole, "descriptor.prepare-node-key", "Descriptor artifact preparation refused"},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			if got := tc.role.operation("prepare-node-key"); got != tc.operation {
				t.Fatalf("receipt operation = %q, want %q", got, tc.operation)
			}
			err := tc.role.failure(cause)
			if !errors.Is(err, cause) || err.Error() != tc.refusal {
				t.Fatalf("role refusal = %v, want %q with original cause retained", err, tc.refusal)
			}
		})
	}
}
