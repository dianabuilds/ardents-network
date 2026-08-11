package namedsite

import (
	"fmt"
	"strings"
	"testing"
)

func TestFailedReferenceRoleRequiresExactRoleSet(t *testing.T) {
	ids := make([]string, 0, len(referenceRoles))
	states := make([]string, 0, len(referenceRoles))
	for index, role := range referenceRoles {
		id := fmt.Sprintf("container-%d", index)
		ids = append(ids, id)
		running, exitCode := true, 0
		if role == "administration" {
			running, exitCode = false, 1
		}
		states = append(states, fmt.Sprintf(`{"Id":%q,"Config":{"Labels":{"com.docker.compose.service":%q}},"State":{"Running":%t,"ExitCode":%d}}`, id, role, running, exitCode))
	}
	parsed, err := parseReferenceRoleStates([]byte("["+strings.Join(states, ",")+"]"), ids)
	if err != nil {
		t.Fatal(err)
	}
	failed := firstFailedReferenceRole(parsed)
	if failed == nil || failed.Config.Labels["com.docker.compose.service"] != "administration" || failed.State.ExitCode != 1 {
		t.Fatalf("failed role = %#v", failed)
	}
}

func TestFailedReferenceRoleRejectsMissingOrSwappedIdentity(t *testing.T) {
	data := []byte(`[{"Id":"unexpected","Config":{"Labels":{"com.docker.compose.service":"authority"}},"State":{"Running":false,"ExitCode":1}}]`)
	if _, err := parseReferenceRoleStates(data, []string{"expected"}); err == nil {
		t.Fatal("unexpected container identity was accepted")
	}
}

func TestBoundedRoleLogLimitsOneLargeLine(t *testing.T) {
	input := strings.Repeat("x", maximumRoleErrorBytes*2)
	got, err := collectBoundedRoleLog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > maximumRoleErrorBytes+len(" [truncated]") || !strings.HasSuffix(got, " [truncated]") {
		t.Fatalf("bounded log length/suffix = %d, %q", len(got), got[len(got)-20:])
	}
}
