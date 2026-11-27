package architecture

import (
	"strings"
	"testing"
)

// The retired predecessor directories cannot import current code or be
// imported by it. The separate Route migration check still applies.
func retiredPackageImport(source, local string, localImport bool) bool {
	for _, directory := range []string{"internal/successor", "internal/nodeidentity", "internal/platform/installation"} {
		if strings.HasPrefix(source, directory+"/") || localImport && (local == directory || strings.HasPrefix(local, directory+"/")) {
			return true
		}
	}
	return false
}

// These predecessor files may not inherit permissions after Endpoint reused
// the directory.
func retiredEndpointSource(source string) bool {
	switch source {
	case "internal/domain/endpoint/runtime.go", "internal/domain/endpoint/source_state.go", "internal/domain/endpoint/worker_grant_linux.go":
		return true
	default:
		return false
	}
}

func TestSuccessorRetiredSourceDeliveryHasNoNativeImportGrant(t *testing.T) {
	const delivery = "internal/domain/release/infrastructure/"
	for _, edge := range []struct{ source, dependency string }{
		{delivery + "source_start_linux.go", modulePath + "/internal/platform/systemd"},
		{delivery + "source_replace_linux.go", modulePath + "/internal/domain/network/state"},
		{delivery + "source_recovery_completed_linux.go", modulePath + "/internal/platform/cgroup"},
		{delivery + "source_stop_linux.go", modulePath + "/internal/platform/process"},
		{delivery + "source_transition_scope_linux.go", "golang.org/x/sys/unix"},
	} {
		if successorImportAllowed(edge.source, edge.dependency) {
			t.Errorf("retired Source delivery edge %s -> %s is still permitted", edge.source, edge.dependency)
		}
	}
}
