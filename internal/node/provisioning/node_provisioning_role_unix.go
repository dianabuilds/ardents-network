//go:build linux || darwin

package provisioning

import "github.com/dianabuilds/ardents-network/internal/platform/artifacts"

// Provisioning shares a mechanism, while the entry point fixes the role and
// therefore the independent declarations, key purpose and receipt namespace.
type nodeProvisioningRole string

const (
	forwardingProvisioningRole nodeProvisioningRole = "forwarding"
	joinProvisioningRole       nodeProvisioningRole = "join"
	descriptorProvisioningRole nodeProvisioningRole = "descriptor"
)

func (role nodeProvisioningRole) valid() bool {
	return role == forwardingProvisioningRole || role == joinProvisioningRole || role == descriptorProvisioningRole
}

func (role nodeProvisioningRole) schema(purpose string) string {
	return "ardents-" + string(role) + "-" + purpose + "-v1"
}

func (role nodeProvisioningRole) operation(operation string) string {
	return string(role) + "." + operation
}

func (role nodeProvisioningRole) failure(err error) error {
	switch role {
	case forwardingProvisioningRole:
		return artifacts.Refuse(err, "forwarding artifact preparation refused")
	case joinProvisioningRole:
		return artifacts.Refuse(err, "JOIN artifact preparation refused")
	case descriptorProvisioningRole:
		return artifacts.Refuse(err, "Descriptor artifact preparation refused")
	default:
		return err
	}
}
