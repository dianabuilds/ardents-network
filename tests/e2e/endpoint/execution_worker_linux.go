//go:build linux && document_worker_installed

package endpoint

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// StopInstalledExecutionWorker verifies and stops the sole active installed worker of the selected role.
func StopInstalledExecutionWorker(t *testing.T, ctx context.Context, role string) {
	t.Helper()
	if role != "reader" && role != "publisher" {
		t.Fatal("invalid environment: unknown fixed worker role")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := func(args ...string) string {
		t.Helper()
		child := exec.CommandContext(bounded, "/usr/bin/systemctl", args...)
		child.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
		body, err := child.Output()
		if err != nil {
			t.Fatal("independent original worker observation/stop", err)
		}
		return strings.TrimSpace(string(body))
	}
	listing := command("--system", "--no-pager", "--plain", "--no-legend", "list-units", "--state=active", "ardents-text-"+role+"@*.service")
	rows := strings.Split(listing, "\n")
	if len(rows) != 1 {
		t.Fatal("loss fixture requires exactly one active original worker")
	}
	fields := strings.Fields(rows[0])
	if len(fields) == 0 || !regexp.MustCompile(`^ardents-text-`+role+`@[0-9]+-[1-9][0-9]*-[1-9][0-9]*\.service$`).MatchString(fields[0]) {
		t.Fatal("loss fixture worker identity is unavailable")
	}
	unit := fields[0]
	observe := func() string {
		return command("--system", "show", unit, "-p", "InvocationID", "-p", "MainPID", "-p", "ControlGroup")
	}
	original := observe()
	facts := make(map[string]string)
	for _, row := range strings.Split(original, "\n") {
		name, value, ok := strings.Cut(row, "=")
		if _, duplicate := facts[name]; !ok || duplicate || name == "" || value == "" {
			t.Fatal("loss fixture original worker observation is malformed")
		}
		facts[name] = value
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(facts["InvocationID"]) ||
		!regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(facts["MainPID"]) ||
		facts["ControlGroup"] != "/system.slice/"+unit || original != observe() {
		t.Fatal("loss fixture original worker changed before stop")
	}
	command("--system", "--no-ask-password", "--no-pager", "stop", unit)
}
