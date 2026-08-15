package recoverysmoke

import (
	"slices"
	"testing"
)

func TestEndpointRestartFaultHasBoundedStopGrace(t *testing.T) {
	want := []string{"restart", "--timeout", "1", "publisher-endpoint"}
	if got := endpointRestartArguments(); !slices.Equal(got, want) {
		t.Fatalf("restart arguments=%v want=%v", got, want)
	}
}
