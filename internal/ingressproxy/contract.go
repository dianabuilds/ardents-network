package ingressproxy

import (
	_ "embed"
	"strings"
)

// ProtocolLabel identifies the compatibility version on a proxy image.
const ProtocolLabel = "io.ardents.ingress.protocol"

//go:embed protocol_version.txt
var embeddedProtocolVersion string

// ProtocolVersion returns the compatibility contract required by the node executor.
func ProtocolVersion() string {
	return strings.TrimSpace(embeddedProtocolVersion)
}
