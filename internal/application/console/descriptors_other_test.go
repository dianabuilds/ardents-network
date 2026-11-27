//go:build !linux

package console

import "testing"

func TestOpenRefusesUnsupportedHost(t *testing.T) {
	input, output, err := Open()
	if input != nil || output != nil || err == nil || err.Error() != "document UI requires the selected Linux host" {
		t.Fatalf("unsupported console returned descriptors or wrong refusal: %v, %v, %v", input, output, err)
	}
}
