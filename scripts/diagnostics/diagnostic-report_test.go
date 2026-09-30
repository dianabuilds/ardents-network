//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func reportFixture(t *testing.T, exit int) string {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC()
	manifest := map[string]any{"schema": "ardents-local-diagnostics-v1", "source_sha": strings.Repeat("a", 40), "source_tree_sha256": strings.Repeat("b", 64), "image": "sha256:" + strings.Repeat("c", 64), "go": "go version go1.26.8 linux/amd64", "timeout": "2m"}
	s := summary{Started: now.Add(-time.Minute), Finished: now.Add(-50 * time.Second), ExitCode: exit, Counts: map[string]uint64{}, Peak: map[string]float64{}}
	for name, value := range map[string]any{"manifest.json": manifest, "summary.json": s} {
		if err := writeJSON(filepath.Join(dir, name), value); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"events.ndjson", "samples.ndjson"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func reportRow(t *testing.T, dir, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, werr := f.Write(append(body, '\n'))
	cerr := f.Close()
	if werr != nil || cerr != nil {
		t.Fatal(werr, cerr)
	}
	body, err = os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sum summary
	if err := json.Unmarshal(body, &sum); err != nil {
		t.Fatal(err)
	}
	if name == "samples.ndjson" {
		sum.Samples++
	} else {
		row, _ := json.Marshal(value)
		var ev event
		if err := json.Unmarshal(row, &ev); err != nil {
			t.Fatal(err)
		}
		sum.Counts[ev.Kind]++
	}
	body, err = json.Marshal(sum)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestAssistantFailureIsObservationAndCleanupRemainsSeparate(t *testing.T) {
	dir := reportFixture(t, 2)
	now := time.Now().UTC()
	reportRow(t, dir, "events.ndjson", event{ObservedAt: now, Schema: "ardents-headless-runtime-event-v1", Kind: "headless-runtime-connection-operation-failed", Failure: "caller-context"})
	reportRow(t, dir, "events.ndjson", event{ObservedAt: now, Schema: "ardents-source-event-v1", Kind: "source-failed", Failure: "cleanup"})
	reportRow(t, dir, "events.ndjson", event{ObservedAt: now, Schema: "ardents-headless-runtime-event-v1", Kind: "headless-runtime-ready"})
	r := assessRun(dir, now)
	if r.Status != "command-failed" || r.FirstObserved == nil || r.FirstObserved.Record != 1 || len(r.Failures) != 2 || r.Failures[1].Category != "cleanup" {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(strings.Join(r.Gaps, " "), "current-capability-readiness-not-established") {
		t.Fatal("historical READY became current readiness")
	}
	var text bytes.Buffer
	if err := writeAssistantText(&text, assistantReport{Run: r}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "cleanup") || !strings.Contains(text.String(), "не доказанная первопричина") {
		t.Fatal(text.String())
	}
}
func TestAssistantReadsFirstEventOutsideOldTail(t *testing.T) {
	dir := reportFixture(t, 1)
	reportRow(t, dir, "events.ndjson", event{ObservedAt: time.Now(), Schema: "ardents-source-event-v1", Kind: "source-failed", Failure: "background-work"})
	for range 300 {
		reportRow(t, dir, "events.ndjson", event{ObservedAt: time.Now(), Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "READY"})
	}
	r := assessRun(dir, time.Now())
	if r.FirstObserved == nil || r.FirstObserved.Record != 1 {
		t.Fatalf("earliest retained event lost: %+v", r)
	}
}
func TestAssistantRejectsInvalidAndTruncatedEvidenceWithoutLeaking(t *testing.T) {
	for _, body := range []string{
		`{"started":"SECRET"}`,
		`{"exit_code":0,"exit_code":2}`,
		`{"started":"2026-09-30T00:00:00Z","exit_code":0`,
	} {
		dir := reportFixture(t, 0)
		if err := os.WriteFile(filepath.Join(dir, "summary.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		r := assessRun(dir, time.Now())
		encoded, _ := json.Marshal(r)
		if r.Status != "unavailable" || r.Complete || bytes.Contains(encoded, []byte("SECRET")) {
			t.Fatalf("%s", encoded)
		}
	}
	dir := reportFixture(t, 0)
	if err := os.WriteFile(filepath.Join(dir, "events.ndjson"), []byte(`{"kind":"SECRET"}`), 0600); err != nil {
		t.Fatal(err)
	}
	r := assessRun(dir, time.Now())
	if r.Complete || r.Status != "command-completed-with-incomplete-evidence" {
		t.Fatalf("%+v", r)
	}
}
func TestAssistantUnknownFieldsAndCategoriesCannotEscapeProjection(t *testing.T) {
	dir := reportFixture(t, 2)
	reportRow(t, dir, "events.ndjson", map[string]any{"observed_at": time.Now(), "schema": "ardents-headless-runtime-event-v1", "kind": "headless-runtime-failed", "failure": "SECRET", "target": "SECRET"})
	reportRow(t, dir, "samples.ndjson", map[string]any{"at": time.Now(), "rss_bytes": 42, "container_cgroup": map[string]uint64{"SECRET": 99, "memory.events:max": 3}, "unavailable": []string{"SECRET"}, "extra": "SECRET"})
	r := assessRun(dir, time.Now())
	body, _ := json.Marshal(r)
	if bytes.Contains(body, []byte("SECRET")) || r.Complete || r.ResourceFacts["memory.events:max"] != 3 {
		t.Fatalf("%s", body)
	}
	samples, err := safeSamplesForPanel(dir)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(samples)
	if bytes.Contains(body, []byte("SECRET")) {
		t.Fatalf("%s", body)
	}
	s := summary{Counts: map[string]uint64{"SECRET": 9}, Peak: map[string]float64{"SECRET": 9}}
	clean, err := safeSummaryForPanel(s)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(clean)
	if bytes.Contains(body, []byte("SECRET")) {
		t.Fatal("summary leak")
	}
}
func TestAssistantMemoryMaxWithoutOOMIsOnlyContainerFact(t *testing.T) {
	dir := reportFixture(t, 0)
	reportRow(t, dir, "samples.ndjson", processSample{At: time.Now(), Cgroup: map[string]uint64{"memory.events:max": 7, "memory.events:oom": 0, "memory.events:oom_kill": 0, "memory.max": 4096}, RSSBytes: 100})
	r := assessRun(dir, time.Now())
	if r.ResourceFacts["memory.events:max"] != 7 || r.ResourceFacts["memory.events:oom_kill"] != 0 {
		t.Fatalf("%+v", r)
	}
	found := false
	for _, c := range r.NextChecks {
		if c.ID == "resource-control" {
			found = true
		}
	}
	if !found {
		t.Fatal("hard-limit evidence not explained")
	}
	if r.FirstObserved != nil {
		t.Fatal("resource counter invented owner failure")
	}
}
func TestAssistantComparisonKeepsFailAndRefusesSpeedup(t *testing.T) {
	failed := reportFixture(t, 2)
	passed := reportFixture(t, 0)
	a := buildAssistantReport(passed, failed, time.Now())
	found := false
	for _, c := range a.Comparison.Other.NextChecks {
		if c.ID == "owner-observation-control" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing owner evidence offers no next observation")
	}
	if a.Comparison == nil || a.Comparison.Other.ExitCode != 2 || a.Comparison.Conditions != "unknown" {
		t.Fatalf("%+v", a)
	}
	var text bytes.Buffer
	if err := writeAssistantText(&text, a); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "exit=2") || !strings.Contains(text.String(), "Ускорение") && !strings.Contains(text.String(), "ускорении") {
		t.Fatal(text.String())
	}
	body, err := os.ReadFile(filepath.Join(failed, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	m["go"] = "go version go1.26.7 linux/amd64"
	body, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(failed, "manifest.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	a = buildAssistantReport(passed, failed, time.Now())
	if a.Comparison.Conditions != "different" || len(a.Comparison.Differences) != 1 || a.Comparison.Differences[0] != "compiler" {
		t.Fatalf("%+v", a)
	}
}
func TestAssistantLiveFreshnessAndSourceChangeAreSeparate(t *testing.T) {
	dir := reportFixture(t, 0)
	body, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s summary
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatal(err)
	}
	s.Finished = time.Time{}
	s.ExitCode = -1
	if err := os.Remove(filepath.Join(dir, "summary.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(dir, "live.json"), s); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "live.json"), old, old); err != nil {
		t.Fatal(err)
	}
	r := assessRun(dir, time.Now())
	if r.Status != "observation-stale" || r.Finished || r.Complete {
		t.Fatalf("%+v", r)
	}
	s.Finished = time.Now()
	s.SourceChanged = true
	s.ExitCode = 0
	if err := writeJSON(filepath.Join(dir, "summary.json"), s); err != nil {
		t.Fatal(err)
	}
	r = assessRun(dir, time.Now())
	if !r.SourceChanged || r.Complete || r.Status != "command-completed-with-incomplete-evidence" {
		t.Fatalf("%+v", r)
	}
}
func TestAssistantSpecialFilesAndSymlinksRefuseWithoutBlocking(t *testing.T) {
	for _, name := range []string{"summary.json", "events.ndjson", "samples.ndjson"} {
		dir := reportFixture(t, 0)
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(dir, name), 0600); err != nil {
			t.Fatal(err)
		}
		done := make(chan runReport, 1)
		go func() { done <- assessRun(dir, time.Now()) }()
		select {
		case r := <-done:
			if r.Complete {
				t.Fatal("FIFO accepted")
			}
		case <-time.After(time.Second):
			t.Fatal("FIFO blocked")
		}
	}
	dir := reportFixture(t, 0)
	if err := os.Remove(filepath.Join(dir, "summary.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(reportFixture(t, 0), "summary.json"), filepath.Join(dir, "summary.json")); err != nil {
		t.Fatal(err)
	}
	if r := assessRun(dir, time.Now()); r.Status != "unavailable" || r.Complete {
		t.Fatalf("%+v", r)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	if r := assessRun(alias, time.Now()); r.Status != "unavailable" {
		t.Fatal("directory symlink accepted")
	}
}
func TestAssistantCLIAndPanelUseSameExplanation(t *testing.T) {
	dir := reportFixture(t, 7)
	expected := buildAssistantReport(dir, "", time.Now())
	w := httptest.NewRecorder()
	handler(dir).ServeHTTP(w, httptest.NewRequest("GET", "/report?dir=/etc", nil))
	var actual assistantReport
	if err := json.Unmarshal(w.Body.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || actual.Run.Status != expected.Run.Status || actual.Run.ExitCode != 7 {
		t.Fatal(w.Body.String())
	}
	for _, path := range []string{"/manifest.json", "/command.json", "/events.ndjson", "/report/../../etc/passwd"} {
		w = httptest.NewRecorder()
		handler(dir).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code == 200 {
			t.Fatalf("raw/control route admitted: %s", path)
		}
	}
	w = httptest.NewRecorder()
	handler(dir).ServeHTTP(w, httptest.NewRequest("POST", "/report", nil))
	if w.Code != 405 {
		t.Fatal("read-only boundary lost")
	}
}
func TestAssistantJSONDepthDuplicateAndBounds(t *testing.T) {
	if validJSONRecord([]byte(`{"exit_code":2,"EXIT_CODE":0}`)) || validJSONRecord([]byte(`{"nested":{"x":1,"x":2}}`)) || validJSONRecord([]byte(strings.Repeat("[", 33)+"0"+strings.Repeat("]", 33))) {
		t.Fatal("ambiguous or too-deep input admitted")
	}
	dir := reportFixture(t, 0)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), bytes.Repeat([]byte(" "), lineLimit+1), 0600); err != nil {
		t.Fatal(err)
	}
	r := assessRun(dir, time.Now())
	if r.Complete {
		t.Fatal("oversize manifest admitted")
	}
}

func TestAssistantUnicodeAliasesCannotOverrideEvidence(t *testing.T) {
	for _, key := range []string{"\u017fource_tree_changed", "\u017famples"} {
		t.Run(key, func(t *testing.T) {
			dir := reportFixture(t, 0)
			path := filepath.Join(dir, "summary.json")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var original summary
			if err := json.Unmarshal(body, &original); err != nil {
				t.Fatal(err)
			}
			original.SourceChanged = true
			original.Samples = 1
			body, err = json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			value := "false"
			if strings.HasSuffix(key, "amples") {
				value = "0"
			}
			alias := append(bytes.TrimSpace(body)[:len(bytes.TrimSpace(body))-1], []byte(`,"`+key+`":`+value+`}`)...)
			if validJSONRecord(alias) {
				t.Fatal("Unicode alias admitted")
			}
			if err := os.WriteFile(path, alias, 0600); err != nil {
				t.Fatal(err)
			}
			if r := assessRun(dir, time.Now()); r.Complete {
				t.Fatal("ambiguous summary became complete")
			}
		})
	}
}

func TestAssistantMissingRowsCannotBecomeComplete(t *testing.T) {
	dir := reportFixture(t, 0)
	reportRow(t, dir, "events.ndjson", event{ObservedAt: time.Now(), Schema: "ardents-source-event-v1", Kind: "source-failed", Failure: "cleanup"})
	reportRow(t, dir, "samples.ndjson", processSample{At: time.Now(), RSSBytes: 10})
	if err := os.WriteFile(filepath.Join(dir, "events.ndjson"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "samples.ndjson"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	r := assessRun(dir, time.Now())
	if r.Complete || r.FirstObserved != nil {
		t.Fatalf("%+v", r)
	}
	gaps := strings.Join(r.Gaps, " ")
	if !strings.Contains(gaps, "event-history-count-mismatch") || !strings.Contains(gaps, "resource-sample-count-mismatch") {
		t.Fatal(gaps)
	}
}
func TestAssistantToolInventoryProjectsOnlyKnownVersions(t *testing.T) {
	dir := reportFixture(t, 0)
	inventory := "staticcheck 2025.1.1 (0.6.1)\nScanner: govulncheck@v1.1.4\nPowerShell 7.6.6\nSECRET address SECRET\nmod github.com/kisielk/errcheck v1.20.0\nmod github.com/go-delve/delve v1.27.2\n"
	if err := os.WriteFile(filepath.Join(dir, "tools.txt"), []byte(inventory), 0600); err != nil {
		t.Fatal(err)
	}
	r := assessRun(dir, time.Now())
	body, _ := json.Marshal(r)
	if len(r.Conditions.Tools) != 5 || r.Conditions.Tools["errcheck"] != "v1.20.0" || bytes.Contains(body, []byte("SECRET")) {
		t.Fatalf("%s", body)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.txt"), []byte(inventory+"Scanner: govulncheck@v1.1.3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r = assessRun(dir, time.Now())
	if r.Conditions.Tools["govulncheck"] != "" || !strings.Contains(strings.Join(r.Gaps, " "), "tool-versions-ambiguous") {
		t.Fatal("contradictory tool version retained")
	}
}
func TestAssistantDeclaredModeAndProfilingAreCaptured(t *testing.T) {
	source := t.TempDir()
	dir := filepath.Join(t.TempDir(), "run")
	on := true
	if err := superviseWithConditions(dir, source, []string{"sh", "-c", "exit 7"}, 5*time.Second, false, reportConditions{Mode: "test", Race: &on, Profiling: &on}); err == nil {
		t.Fatal("command failure hidden")
	}
	r := assessRun(dir, time.Now())
	if r.Status != "command-failed" || r.ExitCode != 7 || r.Conditions.Mode != "test" || r.Conditions.Race == nil || !*r.Conditions.Race || r.Conditions.Profiling == nil || !*r.Conditions.Profiling {
		t.Fatalf("%+v", r)
	}
	found := false
	for _, c := range r.NextChecks {
		if c.ID == "unprofiled-control" {
			found = true
		}
	}
	if !found {
		t.Fatal("profiling overhead ignored")
	}
}
func TestAssistantNormalExitDoesNotInventOwnerFailure(t *testing.T) {
	dir := reportFixture(t, 0)
	reportRow(t, dir, "events.ndjson", event{ObservedAt: time.Now(), Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "EXIT"})
	r := assessRun(dir, time.Now())
	if !r.Complete || r.FirstObserved != nil {
		t.Fatalf("%+v", r)
	}
}

func TestAssistantObservedZeroAndPSIPressureAreVisible(t *testing.T) {
	dir := reportFixture(t, 0)
	reportRow(t, dir, "samples.ndjson", processSample{At: time.Now(), Cgroup: map[string]uint64{"memory.events:max": 5, "memory.events:oom": 0, "memory.events:oom_kill": 0}, Pressure: map[string]float64{"cpu": 0, "memory": 12.5, "io": 3.5}})
	r := assessRun(dir, time.Now())
	for _, key := range []string{"memory.events:oom", "memory.events:oom_kill"} {
		value, ok := r.ResourceFacts[key]
		if !ok || value != 0 {
			t.Fatalf("observed zero missing: %s %+v", key, r)
		}
	}
	if cpu, ok := r.PressurePeaks["cpu"]; !ok || cpu != 0 || r.PressurePeaks["memory"] != 12.5 {
		t.Fatalf("PSI evidence missing: %+v", r)
	}
}
func TestAssistantComparisonIncludesMemoryAndTaskBudgets(t *testing.T) {
	before := reportFixture(t, 0)
	after := reportFixture(t, 0)
	reportRow(t, before, "samples.ndjson", processSample{At: time.Now(), Cgroup: map[string]uint64{"memory.max": 4 << 30, "pids.max": 256, "memory.events:oom_kill": 0}})
	reportRow(t, after, "samples.ndjson", processSample{At: time.Now(), Cgroup: map[string]uint64{"memory.max": 8 << 30, "pids.max": 512, "memory.events:oom_kill": 1}})
	r := buildAssistantReport(after, before, time.Now())
	diff := strings.Join(r.Comparison.Differences, " ")
	if r.Comparison.Conditions != "different" || !strings.Contains(diff, "memory-budget") || !strings.Contains(diff, "task-budget") {
		t.Fatalf("%+v", r.Comparison)
	}
	if r.Comparison.ResourceChanges["memory.events:oom_kill"].Before != 0 || r.Comparison.ResourceChanges["memory.events:oom_kill"].After != 1 {
		t.Fatal("counter differences missing")
	}
	var out bytes.Buffer
	if err := writeAssistantText(&out, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "memory.max") || !strings.Contains(out.String(), "pids.max") {
		t.Fatal(out.String())
	}
}
