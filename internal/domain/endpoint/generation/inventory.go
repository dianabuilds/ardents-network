package generation

import "strings"

// Names returns a detached closed generation inventory, not authenticated bytes.
func Names() []string {
	return strings.Fields("ardents-linux-amd64 ardents-text-linux-amd64 ardents-text-reader@.service ardents-text-publisher@.service ardents-text-reader.socket ardents-text-publisher.socket 50-ardents-text.rules ardents-text.conf ardents-endpoint.service protected-endpoint.json request.json headless.json source.json endpoint-unit.service binding.json")
}
