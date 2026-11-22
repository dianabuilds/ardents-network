package main

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
		return &forwardingControlFailure{err}
	case joinProvisioningRole:
		return &joinControlFailure{err}
	case descriptorProvisioningRole:
		return &descriptorControlFailure{err}
	default:
		return err
	}
}
