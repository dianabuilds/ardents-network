//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func newProjection(t *testing.T) *projection {
	t.Helper()
	f, err := openBounded(t.TempDir(), "events.ndjson", recordLimit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return &projection{events: f, summary: summary{Counts: map[string]uint64{}, Peak: map[string]float64{}}}
}
func TestProjectionDropsSensitiveFieldsAndUnknownCategories(t *testing.T) {
	p := newProjection(t)
	p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"lifecycle","state":"READY","reason":"SECRET","generation":"SECRET","assignment_digest":[1],"target":"SECRET","resource":{"fds":12,"SECRET":99},"carrier_profile":"ardents-carrier-quic-v2"}`))
	body, _ := json.Marshal(p.summary)
	if bytes.Contains(body, []byte("SECRET")) || p.summary.Counts["lifecycle"] != 1 || p.summary.Peak["fds"] != 12 || p.summary.LastEvents[0].Carrier != "ardents-carrier-quic-v2" {
		t.Fatalf("projection: %s", body)
	}
	p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"SECRET"}`))
	if p.summary.UnrecognizedLines != 1 || len(p.summary.Counts) != 1 {
		t.Fatal("unknown metric label admitted")
	}
}
func TestJournalAndGoTestWrappers(t *testing.T) {
	p := newProjection(t)
	source := `{"schema":"ardents-source-event-v1","kind":"source-failed","reason":"cleanup","at":"2026-09-30T00:00:00Z"}`
	journal, _ := json.Marshal(map[string]string{"MESSAGE": source, "_PID": "SECRET"})
	p.line(journal)
	wrapper, _ := json.Marshal(map[string]string{"Action": "output", "Output": source + "\n"})
	p.line(wrapper)
	if p.summary.Counts["source-failed"] != 2 {
		t.Fatal("wrappers not recognized")
	}
}
func TestOversizedLineDoesNotBlockFollowingEvent(t *testing.T) {
	p := newProjection(t)
	body := strings.Repeat("x", lineLimit+100) + "\n" + `{"schema":"ardents-node-event-v1","kind":"lifecycle","state":"FAILED"}` + "\n"
	if err := drain(strings.NewReader(body), nil, p, true); err != nil {
		t.Fatal(err)
	}
	if p.summary.OversizedLines != 1 || p.summary.Counts["lifecycle"] != 1 {
		t.Fatalf("summary: %+v", p.summary)
	}
}
func TestRawSaturationDrainsAndRetainsLoss(t *testing.T) {
	f, err := openBounded(t.TempDir(), "raw.log", 3)
	if err != nil {
		t.Fatal(err)
	}
	n, err := f.Write([]byte("123456"))
	if err != nil || n != 6 || f.written != 3 || f.dropped != 3 {
		t.Fatalf("bounded writer: %+v, %v", f, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestSinkFailureDoesNotStopPipeDraining(t *testing.T) {
	f, err := openBounded(t.TempDir(), "raw.log", 100)
	if err != nil {
		t.Fatal(err)
	}
	f.file.Close()
	p := newProjection(t)
	if err := drain(strings.NewReader("sensitive output"), f, p, false); err != nil {
		t.Fatal(err)
	}
	if f.failure == nil || p.summary.StderrBytes != 16 {
		t.Fatal("sink failure lost or producer blocked")
	}
	if f.dropped != 16 || p.summary.RawDroppedBytes != 16 || !p.summary.CaptureIncomplete {
		t.Fatalf("failed sink lost byte accounting: dropped=%d summary=%+v", f.dropped, p.summary)
	}
	if err := drain(strings.NewReader("more"), f, p, false); err != nil {
		t.Fatal(err)
	}
	if f.dropped != 20 || p.summary.RawDroppedBytes != 20 || p.summary.StderrBytes != 20 {
		t.Fatalf("discard after sink failure was not counted: dropped=%d summary=%+v", f.dropped, p.summary)
	}
}
func TestFailureAndTimeoutHaveTerminalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		limit   time.Duration
		exit    int
		timeout bool
	}{
		{"failed", "exit 7", 2 * time.Second, 7, false}, {"timeout", "trap '' TERM; sleep 30", 100 * time.Millisecond, -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			dir := filepath.Join(t.TempDir(), "capture")
			if err := supervise(dir, source, []string{"sh", "-c", tc.command}, tc.limit, false); err == nil {
				t.Fatal("failure passed")
			}
			s, err := readSummary(dir)
			if err != nil {
				t.Fatal(err)
			}
			if s.ExitCode != tc.exit || s.TimedOut != tc.timeout {
				t.Fatalf("terminal outcome: %+v", s)
			}
			if _, err := os.Stat(filepath.Join(dir, "stdout.log")); !os.IsNotExist(err) {
				t.Fatal("raw capture silently enabled")
			}
		})
	}
}
func TestDashboardCannotServeRawFilesOrInjectMetricLabels(t *testing.T) {
	dir := t.TempDir()
	s := summary{Counts: map[string]uint64{`SECRET"} 1`: 99}, Peak: map[string]float64{"SECRET": 5}}
	if err := writeJSON(filepath.Join(dir, "summary.json"), s); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	handler(dir).ServeHTTP(r, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(r.Body.String(), "SECRET") {
		t.Fatal("dynamic labels escaped allowlist")
	}
	r = httptest.NewRecorder()
	handler(dir).ServeHTTP(r, httptest.NewRequest("GET", "/stdout.log", nil))
	if r.Code != 404 {
		t.Fatal("raw file exposed")
	}
}
func TestEvidenceWithinSourceRefused(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := collect("run", []string{"-out", filepath.Join(cwd, "bad-capture"), "--", "true"}); err == nil {
		t.Fatal("source evidence admitted")
	}
}
func TestEventTailRemainsBounded(t *testing.T) {
	p := newProjection(t)
	for range 1000 {
		p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"lifecycle"}`))
	}
	if len(p.summary.LastEvents) != 256 || p.summary.Counts["lifecycle"] != 1000 {
		t.Fatal("event tail or counters incorrect")
	}
}
func TestSourceDigestIncludesUncommittedContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "untracked"), []byte("one"), 0600)
	first := sourceDiffDigest(context.Background(), dir)
	os.WriteFile(filepath.Join(dir, "untracked"), []byte("two"), 0600)
	if first == "unavailable" || first == sourceDiffDigest(context.Background(), dir) {
		t.Fatal("source content identity lost")
	}
}

func TestSuccessfulCommandRetainsCompleteTerminalEvidence(t *testing.T) {
	source := t.TempDir()
	dir := filepath.Join(t.TempDir(), "capture")
	if err := supervise(dir, source, []string{"sh", "-c", "echo '{\"schema\":\"ardents-node-event-v1\",\"kind\":\"lifecycle\",\"state\":\"READY\"}'"}, 2*time.Second, false); err != nil {
		t.Fatal(err)
	}
	outcome, err := readSummary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != 0 || outcome.CaptureIncomplete || outcome.Counts["lifecycle"] != 1 {
		t.Fatalf("successful command evidence: %+v", outcome)
	}
}

func TestTimingsKeepFailedTestsAndExcludeOverlappingSubtests(t *testing.T) {
	input := `{"Action":"pass","Package":"owner","Test":"TestParent/Child","Elapsed":9}
{"Action":"fail","Package":"owner","Test":"TestParent","Elapsed":10}
{"Action":"fail","Package":"owner","Elapsed":11}
`
	var output bytes.Buffer
	if err := writeTestTimings(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Child") || !strings.Contains(output.String(), "10.000s fail owner TestParent") {
		t.Fatal(output.String())
	}
	if err := writeTestTimings(strings.NewReader(`{"Action":"run"}`), io.Discard); err == nil {
		t.Fatal("incomplete timing passed")
	}
}

func TestCurrentOwnerCategoriesAndLifecycleTailSurviveResourceSamples(t *testing.T) {
	p := newProjection(t)
	p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"lifecycle","state":"READY"}`))
	p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"resource","state":"PROTECT"}`))
	for i := 0; i < 300; i++ {
		p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"resource-sample","state":"OBSERVED"}`))
	}
	if len(p.summary.LastEvents) != 2 || p.summary.LastEvents[0].State != "READY" || p.summary.LastEvents[1].State != "PROTECT" || p.summary.LastResource.State != "OBSERVED" {
		t.Fatal("resource samples displaced transition or drain state")
	}
	for _, failure := range []string{"startup", "running", "rotation"} {
		event, reason := project([]byte(`{"schema":"ardents-headless-runtime-event-v1","kind":"headless-runtime-failed","failure":"` + failure + `"}`))
		if reason != "" || event.Failure != failure {
			t.Fatalf("failure category lost: %s", failure)
		}
	}
	body := metricText(summary{Interrupted: true, ExitCode: 0})
	if !strings.Contains(body, "ardents_diagnostic_interrupted 1\n") {
		t.Fatal("interruption hidden from metrics")
	}
}
func TestSourceInventoryRejectsFIFOAndHonorsCanceledContext(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() { done <- sourceDiffDigest(context.Background(), root) }()
	select {
	case digest := <-done:
		if digest != "unavailable" {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO blocked inventory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sourceDiffDigest(ctx, t.TempDir()) != "unavailable" {
		t.Fatal("canceled inventory accepted")
	}
}
func TestWholeRunDeadlineIncludesInventoryAndProducesFailureReceipt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "expired")
	if err := supervise(dir, t.TempDir(), []string{"sh", "-c", "exit 0"}, time.Nanosecond, false); err == nil {
		t.Fatal("expired setup accepted")
	}
	body, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result summary
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if !result.TimedOut || !result.CaptureIncomplete || result.ExitCode != -1 {
		t.Fatalf("setup failure not retained: %+v", result)
	}
}

func TestUnsupportedOwnerCategoryIsVisibleWithoutPrivateValue(t *testing.T) {
	p := newProjection(t)
	p.line([]byte(`{"schema":"ardents-headless-runtime-event-v1","kind":"headless-runtime-failed","failure":"PRIVATE"}`))
	body, _ := json.Marshal(p.summary)
	if bytes.Contains(body, []byte("PRIVATE")) || !p.summary.CaptureIncomplete || p.summary.UnknownCategories != 1 || p.summary.LastEvents[0].Failure != "unclassified" {
		t.Fatalf("unsupported category hidden/leaked: %s", body)
	}
}

func TestSourceMutationInvalidatesExactCandidateReceipt(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "payload"), []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "mutation")
	if err := supervise(out, source, []string{"sh", "-c", "printf after > payload"}, time.Second, false); err == nil {
		t.Fatal("changing candidate accepted")
	}
	body, err := os.ReadFile(filepath.Join(out, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result summary
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if !result.SourceChanged || !result.CaptureIncomplete || result.ExitCode != 0 {
		t.Fatal(string(body))
	}
}
