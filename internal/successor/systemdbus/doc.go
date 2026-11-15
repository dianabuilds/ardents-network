// Package systemdbus validates bounded detached system-bus value bytes.
// It opens no connection and grants no manager, process or runtime authority.
package systemdbus

import "github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"

// ErrObservation preserves the original manager refusal identity.
var ErrObservation = systemdproperty.ErrObservation
