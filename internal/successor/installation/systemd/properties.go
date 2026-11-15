package systemd

import "github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"

// Value and Properties preserve the typed facts returned by the original manager.
type Value = systemdproperty.Value
type Properties = systemdproperty.Properties

// Decode validates a detached manager observation.
func Decode(raw []byte, target any) error { return systemdproperty.Decode(raw, target) }

// Matches checks an exact typed property.
func Matches(properties Properties, name, signature string, wanted any) bool {
	return systemdproperty.Matches(properties, name, signature, wanted)
}

// Contains checks an observed string array without admitting duplicate values.
func Contains(properties Properties, name string, required []string) bool {
	return systemdproperty.Contains(properties, name, required)
}
