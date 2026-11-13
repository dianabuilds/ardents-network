package architecture

import "testing"

func TestJoinRecoveryImportScope(t *testing.T) {
	for _, entry := range []struct {
		source, dependency string
		allowed            bool
	}{
		{"internal/successor/installation/join/transition_scope_linux.go", "golang.org/x/sys/unix", true},
		{"internal/successor/installation/join/other_scope_linux.go", "golang.org/x/sys/unix", false},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/successor/release", true},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/successor/installation/systemd", true},
		{"internal/successor/installation/join/other_recovery_linux.go", modulePath + "/internal/successor/release", false},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/successor/installation/forwarding", false},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/successor/node", false},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/node", false},
		{"internal/successor/installation/join/recovery_replacement_linux.go", modulePath + "/internal/successor/route/role", false},
	} {
		if got := successorImportAllowed(entry.source, entry.dependency); got != entry.allowed {
			t.Errorf("%s imports %s: %v, want %v", entry.source, entry.dependency, got, entry.allowed)
		}
	}
}
