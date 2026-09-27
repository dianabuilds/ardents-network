//go:build linux

package endpoint

import (
	"strings"
	"testing"
)

func TestQualificationPublisherIncompleteErrorReportsProgressAndRegistrationReason(t *testing.T) {
	err := qualificationPublisherIncompleteError(193)
	message := err.Error()
	for _, want := range []string{"193/256", "producer ended"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}
