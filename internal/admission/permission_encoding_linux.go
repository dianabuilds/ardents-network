//go:build linux

package admission

// EncodePermission returns the canonical 228-byte signed permission.
func EncodePermission(permission Permission) ([]byte, error) {
	if err := validatePermission(permission); err != nil {
		return nil, err
	}
	raw := permissionUnsigned(permission)
	return append(raw, permission.Signature[:]...), nil
}
