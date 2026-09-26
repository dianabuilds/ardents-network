package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredInitiatorReceivingClosureIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/admission.go",
		"internal/entry/admission_test.go",
		"internal/route/credential/forward.go",
		"internal/route/credential/forward_redirect_test.go",
		"internal/service/reachability/gateway_redirect_test.go",
		"internal/service/reachability/gateway_tls_client.go",
		"internal/service/reachability/private_relay.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Initiator Entry-admission file still exists: %s", relative)
		}
	}
}
