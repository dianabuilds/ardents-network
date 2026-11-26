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

func TestProjectionJournalEnvelopeCannotSupplyEvent(t *testing.T) {
	p := newProjection(t)
	p.line([]byte(`{"schema":"ardents-source-event-v1","kind":"source-ready","MESSAGE":"{}"}`))
	if len(p.summary.Counts) != 0 || p.summary.UnrecognizedLines != 1 {
		t.Fatalf("outer fields created event: %+v", p.summary)
	}
	journal := `{"reason":"private-outer-detail","MESSAGE":"{\"schema\":\"ardents-source-event-v1\",\"kind\":\"source-ready\"}"}`
	p.line([]byte(journal))
	if p.summary.Counts["source-ready"] != 1 || p.summary.UnknownCategories != 0 {
		t.Fatalf("outer reason changed event: %+v", p.summary)
	}
}

func TestProjectionRejectsMalformedKnownRecord(t *testing.T) {
	for _, body := range []string{
		`{"schema":"ardents-source-event-v1","kind":"source-ready","reason":"cleanup"}`,
		`{"schema":"ardents-source-event-v1","kind":"source-failed","reason":"private-marker"}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","state":42}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","at":false}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","at":"not-a-time"}`,
	} {
		p := newProjection(t)
		p.line([]byte(body))
		if p.summary.InvalidLines != 1 || !p.summary.CaptureIncomplete || len(p.summary.Counts) != 0 || len(p.summary.LastEvents) != 0 {
			t.Fatalf("malformed known event accepted: %+v", p.summary)
		}
	}
}

func TestOrdinaryConsoleAndGoTestOutputDoNotDeclareCorruptEvents(t *testing.T) {
	p := newProjection(t)
	for _, line := range []string{"ordinary console output", "=== RUN   TestWorker", "--- PASS: TestWorker (0.01s)"} {
		p.line([]byte(line))
		wrapped, err := json.Marshal(map[string]string{"Action": "output", "Output": line + "\n"})
		if err != nil {
			t.Fatal(err)
		}
		p.line(wrapped)
	}
	if p.summary.UnrecognizedLines != 6 || p.summary.InvalidLines != 0 || p.summary.CaptureIncomplete || len(p.summary.Counts) != 0 {
		t.Fatalf("ordinary output became corrupt event: %+v", p.summary)
	}
	p.line([]byte(`{"schema":"ardents-node-event-v1","kind":"lifecycle"`))
	if p.summary.InvalidLines != 1 || !p.summary.CaptureIncomplete {
		t.Fatalf("corrupt JSON event was hidden: %+v", p.summary)
	}
}

func TestCurrentCommandReportsReachSafeCollectorProjection(t *testing.T) {
	p := newProjection(t)
	for _, raw := range []string{
		`{"operation":"node.forwarding","phase":"listening","outcome":"original-duty","first_failure":"SECRET","resource":{"fds":99},"at":"2030-01-01T00:00:00Z"}`,
		`{"operation":"node.forwarding","phase":"retiring","outcome":"completion-unavailable","first_owner":"SECRET"}`,
		`{"operation":"node.forwarding","phase":"joined","outcome":"failed","cleanup_owner":"SECRET"}`,
		`{"operation":"source.serve","phase":"joined","outcome":"canceled","request_digest":"SECRET"}`,
	} {
		p.line([]byte(raw))
	}
	if len(p.summary.LastEvents) != 4 || p.summary.Counts["node.forwarding"] != 3 || p.summary.CaptureIncomplete {
		t.Fatalf("current reports lost: %+v", p.summary)
	}
	rows := p.summary.LastEvents
	if rows[0].Failure != "" || rows[0].Phase != "listening" || rows[0].Outcome != "original-duty" || rows[0].At != "" || len(rows[0].Resource) != 0 || rows[0].Owner != "node" {
		t.Fatal("listening became failure or invented measurement")
	}
	if rows[1].Failure != "completion-unavailable" || rows[2].Failure != "failed" || rows[3].Failure != "" {
		t.Fatal("unavailable, failed and canceled results conflated")
	}
	for _, row := range rows {
		if !safeReportEvent(row) {
			t.Fatalf("live and saved readers disagree: %+v", row)
		}
	}
	clean, err := safeSummaryForPanel(p.summary)
	raw, _ := json.Marshal(clean)
	if err != nil || bytes.Contains(raw, []byte("SECRET")) {
		t.Fatal("private command detail reached panel", err)
	}
	p.line([]byte(`{"operation":"node.forwarding","phase":"joined","outcome":"original-duty"}`))
	if p.summary.InvalidLines != 1 || !p.summary.CaptureIncomplete {
		t.Fatal("invalid transition hidden")
	}
}

func TestStructuredStderrIsProjectedWithoutRawCapture(t *testing.T) {
	p := newProjection(t)
	stdout := "{\"operation\":\"source.serve\",\"phase\":\"listening\",\"outcome\":\"original-source\"}\n"
	stderr := "{\"operation\":\"source.serve\",\"phase\":\"joined\",\"outcome\":\"failed\",\"first_owner\":\"SECRET\"}\n"
	if err := drain(strings.NewReader(stdout), nil, p, true); err != nil {
		t.Fatal(err)
	}
	if err := drain(strings.NewReader(stderr), nil, p, false); err != nil {
		t.Fatal(err)
	}
	if p.summary.StdoutBytes != int64(len(stdout)) || p.summary.StderrBytes != int64(len(stderr)) || p.summary.Counts["source.serve"] != 2 || len(p.summary.LastEvents) != 2 {
		t.Fatalf("stderr event lost: %+v", p.summary)
	}
	if p.summary.LastEvents[0].Failure != "" || p.summary.LastEvents[1].Failure != "failed" {
		t.Fatal("stream was treated as event severity")
	}
	body, _ := json.Marshal(p.summary)
	if bytes.Contains(body, []byte("SECRET")) {
		t.Fatal("stderr private detail retained")
	}
	if err := drain(strings.NewReader(strings.Repeat("SECRET", lineLimit)+"\n"+stderr), nil, p, false); err != nil {
		t.Fatal(err)
	}
	if p.summary.OversizedLines != 1 || p.summary.Counts["source.serve"] != 3 {
		t.Fatal("oversized stderr stopped later event projection")
	}
}

func TestDomainAndExporterResultsRemainSeparateInProjection(t *testing.T) {
	entry, reason := project([]byte(`{"Operation":"hosting.observe","Phase":"execute","Outcome":"completed","Telemetry":"unavailable"}`))
	if reason != "" || entry.Delivery != "unavailable" || entry.Outcome != "completed" || entry.Failure != "" || !safeReportEvent(entry) {
		t.Fatalf("delivery changed domain outcome: %+v %s", entry, reason)
	}
	entry.Delivery = "PRIVATE"
	if safeReportEvent(entry) {
		t.Fatal("arbitrary delivery label reached panel")
	}
}

func TestInternalMeasurementsUseSharedSafeProjectionAndMetricKinds(t *testing.T) {
	raw := []byte(`{"schema":"ardents-domain-measurements-v1","at":"2026-09-30T00:00:01Z","started_at":"2026-09-30T00:00:00Z","discarded_snapshots":0,"telemetry":"unavailable","target":"PRIVATE","measurements":[{"name":"route.queue_budget_claim_bytes","available":true,"current":7,"peak":9,"count":0,"seconds":0,"maximum_seconds":0,"credential":"PRIVATE"},{"name":"enrollment.verify","available":true,"current":0,"peak":1,"count":2,"seconds":0.015,"maximum_seconds":0.01}]}`)
	projected, reason := project(raw)
	if reason != "" || projected.Measurements == nil || projected.Failure != "" || !safeReportEvent(projected) {
		t.Fatal("measurement projection", reason, projected)
	}
	body, err := json.Marshal(projected)
	if err != nil || bytes.Contains(body, []byte("PRIVATE")) {
		t.Fatal("private data retained", err)
	}
	text := metricText(summary{LastEvents: []event{projected}})
	for _, want := range []string{"# TYPE ardents_domain_route_queue_budget_claim_bytes_current gauge\n", "ardents_domain_route_queue_budget_claim_bytes_current 7\n", "# TYPE ardents_domain_enrollment_verify_calls_total counter\n", "ardents_domain_enrollment_verify_calls_total 2\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	dir := reportFixture(t, 0)
	reportRow(t, dir, "events.ndjson", projected)
	report := assessRun(dir, time.Now().UTC())
	if report.DomainMeasurements == nil || len(report.DomainSeries) != 1 || report.FirstObserved != nil || len(report.UnobservedInstruments) == 0 || !strings.Contains(strings.Join(report.Gaps, " "), "telemetry-export-unavailable") {
		t.Fatalf("measurement availability/result conflated: %+v", report)
	}
	malformed := bytes.Replace(raw, []byte(`"current":7`), []byte(`"current":null`), 1)
	if _, reason := project(malformed); reason != "invalid" {
		t.Fatal("null measurement admitted", reason)
	}
}
