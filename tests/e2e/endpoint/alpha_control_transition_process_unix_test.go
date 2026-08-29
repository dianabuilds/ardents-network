//go:build linux

package endpoint_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAlphaControlTransitionsTwoFreshEnrolledEndpointsAgree(t *testing.T) {
	endpoint := buildArdents(t)
	control := buildControl(t)
	fixture := alphaControlBundle(t, endpoint, control)
	roots := [2]string{freshEndpointProcessRoot(t, "h46b-e1-"), freshEndpointProcessRoot(t, "h46b-e2-")}
	endpoints := [2]*liveEnrolledEndpoint{
		startLiveEnrolledEndpoint(t, fixture.artifact, fixture.input, roots[0], alphaControlEndpointCohort, alphaControlEndpointRelease),
		startLiveEnrolledEndpoint(t, fixture.artifact, fixture.input, roots[1], alphaControlEndpointCohort, alphaControlEndpointRelease),
	}
	var reports [2][]byte
	for index, endpoint := range endpoints {
		if err := endpoint.command.Process.Signal(syscall.Signal(0)); err != nil {
			t.Fatalf("fresh Endpoint %d was not live during transition inspection: %v", index, err)
		}
		reports[index] = inspectLiveEndpointTransitions(t, fixture, filepath.Join(t.TempDir(), "transitions"))
		assertExactAlphaTransitionReport(t, fixture, reports[index])
	}
	if !bytes.Equal(reports[0], reports[1]) {
		t.Fatalf("fresh Endpoint transition reports differ:\n%s\n%s", reports[0], reports[1])
	}
	for index := range endpoints {
		endpoints[index].stop(t)
	}
}

func inspectLiveEndpointTransitions(t *testing.T, fixture alphaControlBundleFixture, stateRoot string) []byte {
	t.Helper()
	arguments := []string{"inspect-transitions", "--enrollment", fixture.input, "--artifact", fixture.artifact,
		"--state-root", stateRoot, "--at", fixture.now.Format(time.RFC3339)}
	output, err := exec.Command(fixture.control, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect live fresh Endpoint transitions: %v\n%s", err, output)
	}
	return output
}

func assertExactAlphaTransitionReport(t *testing.T, fixture alphaControlBundleFixture, raw []byte) {
	t.Helper()
	var report struct {
		Schema      string                        `json:"schema"`
		Control     alphaControlParticipantReport `json:"control"`
		Transitions []struct {
			Domain      string `json:"domain"`
			Selected    bool   `json:"selected"`
			Outcome     string `json:"outcome"`
			UserFailure string `json:"user_failure"`
			Evidence    string `json:"evidence"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode fresh Endpoint transition report: %v\n%s", err, raw)
	}
	if report.Schema != "ardents-alpha-transition-report-v1" {
		t.Fatalf("fresh Endpoint transition schema = %q", report.Schema)
	}
	assertExactAlphaControlReport(t, fixture, report.Control)
	want := []struct {
		domain   string
		selected bool
		outcome  string
	}{
		{"release-safety", true, "accepted"}, {"network-epoch", true, "accepted"},
		{"compatibility", true, "accepted"}, {"namespace-materialization", false, "not-selected"},
	}
	if len(report.Transitions) != len(want) {
		t.Fatalf("fresh Endpoint transition count = %d", len(report.Transitions))
	}
	for index, expected := range want {
		actual := report.Transitions[index]
		if actual.Domain != expected.domain || actual.Selected != expected.selected || actual.Outcome != expected.outcome || actual.UserFailure == "" || actual.Evidence == "" {
			t.Fatalf("fresh Endpoint transition %d = %+v", index, actual)
		}
	}
}
