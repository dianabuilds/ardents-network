//go:build ignore

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMonitorFollowsSourceAndRestartsRetention(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	run := func() monitorState {
		t.Helper()
		if err := dispatch([]string{"monitor", "-out", out, "-console=false", "-raw", "--", "/bin/sh", "-c", "printf 'private-stdout\\n'; printf 'private-stderr\\n' >&2"}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(out, "monitor.json"))
		if err != nil {
			t.Fatal(err)
		}
		var state monitorState
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		if state.SourceAlive || state.Finished == nil || state.SourceExit == nil || *state.SourceExit != 0 || state.FileFailed || state.CleanupFailed || state.Logs.LostBytes != 0 || state.QueueDroppedBytes != 0 {
			t.Fatalf("terminal state: %+v", state)
		}
		if state.StdoutBytes != 15 || state.StderrBytes != 15 || len(state.Tail) != 2 || state.UnknownLines != 2 {
			t.Fatalf("producer accounting: %+v", state)
		}
		for _, row := range state.Tail {
			encoded, _ := json.Marshal(row)
			if strings.Contains(string(encoded), "private-") {
				t.Fatalf("raw text leaked into projection: %s", encoded)
			}
		}
		return state
	}
	first := run()
	second := run()
	if second.Logs.RetainedBytes <= first.Logs.RetainedBytes || second.Logs.ReceivedBytes != second.Logs.WrittenBytes || second.Logs.RetainedBytes != first.Logs.RetainedBytes+second.Logs.WrittenBytes {
		t.Fatalf("restart inventory/session accounting first=%+v second=%+v", first.Logs, second.Logs)
	}
	entries, err := os.ReadDir(filepath.Join(out, "logs"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(out, "logs", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(entry.Name(), "-stdout.log") {
			stdout.Write(body)
		}
		if strings.HasSuffix(entry.Name(), "-stderr.log") {
			stderr.Write(body)
		}
	}
	if stdout.String() != "private-stdout\nprivate-stdout\n" || stderr.String() != "private-stderr\nprivate-stderr\n" {
		t.Fatalf("retained streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestMonitorTimeoutJoinsSourceAndWritesTerminalState(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	root, err := openMonitorRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := openLogStore(filepath.Join(out, "logs"), logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	delivery := newMonitorDelivery(store, nil, false, time.Now().UTC())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = runMonitorSource(ctx, root, delivery, []string{"/bin/sh", "-c", "printf 'started\\n'; sleep 30"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout result: %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("source cancellation exceeded join budget")
	}
	state := delivery.snapshot()
	if state.SourceAlive || state.Finished == nil || state.SourceExit == nil || !state.TimedOut || state.Interrupted || state.CleanupFailed || state.Logs.LostBytes != 0 {
		t.Fatalf("cancel state: %+v", state)
	}
	body, err := os.ReadFile(filepath.Join(out, "monitor.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved monitorState
	if err := json.Unmarshal(body, &saved); err != nil || saved.Finished == nil || !saved.TimedOut || saved.SourceAlive {
		t.Fatalf("terminal snapshot: %s (%v)", body, err)
	}
}

func TestMonitorStartFailureHasTerminalEvidence(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	err := dispatch([]string{"monitor", "-out", out, "-console=false", "--", "/no-such-ardents-monitor-fixture"})
	if err == nil {
		t.Fatal("missing source became success")
	}
	body, readErr := os.ReadFile(filepath.Join(out, "monitor.json"))
	if readErr != nil {
		t.Fatalf("start failure lost terminal evidence: %v", readErr)
	}
	var state monitorState
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if state.SourceAlive || state.Finished == nil || state.SourceExit != nil || !state.StartFailed {
		t.Fatalf("start failure state: %+v", state)
	}
}

func TestMonitorBlockedConsoleKeepsDrainingAndJoins(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	root, err := openMonitorRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := openLogStore(filepath.Join(out, "logs"), logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	consoleR, consoleW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer consoleR.Close()
	delivery := newMonitorDelivery(store, consoleW, false, time.Now().UTC())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = runMonitorSource(ctx, root, delivery, []string{"/bin/sh", "-c", "i=0; while [ $i -lt 5000 ]; do printf 'fixture-line\\n'; i=$((i+1)); done"})
	if err == nil {
		t.Fatal("blocked console became healthy")
	}
	state := delivery.snapshot()
	if state.StdoutBytes != 65000 || state.SourceExit == nil || *state.SourceExit != 0 || state.ConsoleDroppedBytes == 0 || !state.ConsoleFailed {
		t.Fatalf("producer blocked or sink loss concealed: %+v", state)
	}
	if !state.SinksJoined {
		t.Fatalf("console worker failed to join: %+v", state)
	}
}

func TestMonitorLivePanelSurvivesFileFailureAndShowsSourceExit(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	root, err := openMonitorRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	at := time.Now().UTC()
	store, err := openLogStore(filepath.Join(out, "logs"), logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("events", []byte("seed\n"), at); err != nil {
		t.Fatal(err)
	}
	// A real failed file descriptor, established before any producer/worker runs.
	if err := store.active["events"].file.Close(); err != nil {
		t.Fatal(err)
	}
	delivery := newMonitorDelivery(store, nil, false, at)
	view, err := openMonitorView("127.0.0.1:0", false, delivery)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := view.close(); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runMonitorSource(ctx, root, delivery, []string{"/bin/sh", "-c", "printf 'private-fixture-content\\n'; sleep 30"})
	}()
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	address := "http://" + view.listener.Addr().String()
	fetch := func(path, origin string) (int, []byte) {
		t.Helper()
		request, err := http.NewRequest("GET", address+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 65537))
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatal(err)
		}
		if len(body) > 65536 || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("unbounded or cacheable monitor response")
		}
		return response.StatusCode, body
	}
	deadline := time.Now().Add(3 * time.Second)
	var status struct {
		State        monitorState `json:"state"`
		SourceStatus string       `json:"source_status"`
	}
	for {
		code, body := fetch("/status", "")
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, body)
		}
		if strings.Contains(string(body), "private-fixture-content") {
			t.Fatal("private content leaked through HTTP")
		}
		if err := json.Unmarshal(body, &status); err != nil {
			t.Fatal(err)
		}
		if status.State.FileFailed && status.State.SourceAlive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("live failure not visible: %+v", status.State)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status.SourceStatus != "running" || status.State.Logs.LostBytes == 0 || status.State.StdoutBytes != 24 {
		t.Fatalf("live source or sink accounting: %+v", status)
	}
	if code, _ := fetch("/status", "https://foreign.example"); code != http.StatusForbidden {
		t.Fatalf("foreign origin admitted: %d", code)
	}
	if code, _ := fetch("/logs/seed.log", ""); code != http.StatusNotFound {
		t.Fatalf("raw file path admitted: %d", code)
	}
	if code, body := fetch("/", ""); code != http.StatusOK || !strings.Contains(string(body), "Пауза просмотра") {
		t.Fatalf("live UI unavailable: %d", code)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("source result: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("source did not join")
	}
	_, body := fetch("/status", "")
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if status.SourceStatus != "stopped" || status.State.SourceAlive || status.State.SourceExit == nil || status.State.Finished == nil || !status.State.FileFailed || !status.State.SinksJoined {
		t.Fatalf("terminal failure/source evidence erased: %+v", status)
	}
}

func TestMonitorProjectsStructuredStderrWithoutRawContent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	err := dispatch([]string{"monitor", "-out", out, "-console=false", "--", "/bin/sh", "-c", `printf '%s\n' '{"schema":"ardents-node-event-v1","kind":"lifecycle","state":"FAILED","reason":"private-reason","target":"private-target"}' >&2`})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out, "monitor.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state monitorState
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Tail) != 1 || state.Tail[0].Stream != "stderr" || state.Tail[0].Entry.State != "FAILED" || state.UnknownLines != 0 || state.Raw {
		t.Fatalf("structured stderr not projected: %+v", state)
	}
	if strings.Contains(string(body), "private-") {
		t.Fatal("private stderr fields leaked")
	}
}

func TestMonitorNumericTailStaysWithinStatusBudget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	store, err := openLogStore(dir, logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	delivery := newMonitorDelivery(store, nil, false, time.Now().UTC())
	defer func() {
		if err := delivery.close(); err != nil {
			t.Error(err)
		}
	}()
	values := map[string]float64{}
	for _, field := range resourceFields {
		values[field] = math.MaxFloat64
	}
	body, err := json.Marshal(map[string]any{"schema": "ardents-node-event-v1", "kind": "resource", "state": "OBSERVED", "resource": values, "hosting": map[string]any{"Observation": map[string]float64{"UsedBytes": math.MaxFloat64, "ReservedBytes": math.MaxFloat64, "RemainingBytes": math.MaxFloat64}}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64; i++ {
		delivery.row("stdout", body, false)
	}
	request := httptest.NewRequest("GET", "http://127.0.0.1:8094/status", nil)
	response := httptest.NewRecorder()
	monitorHandler(delivery).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() > 65536 {
		t.Fatalf("valid numeric logs disabled status: code=%d bytes=%d", response.Code, response.Body.Len())
	}
	state := delivery.snapshot()
	root, err := os.OpenRoot(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writeMonitorState(root, state); err != nil {
		t.Fatalf("valid numeric rows disabled saved status: %v", err)
	}
	if state.TotalRows != 64 || len(state.Tail) == 0 || state.TailEvictedRows+uint64(len(state.Tail)) != state.TotalRows {
		t.Fatalf("tail accounting: total=%d retained=%d evicted=%d", state.TotalRows, len(state.Tail), state.TailEvictedRows)
	}
}

func TestMonitorKeepsRetentionActiveForTerminalPanel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "monitor")
	done := make(chan error, 1)
	go func() {
		done <- dispatch([]string{"monitor", "-out", out, "-console=false", "-listen", address, "-timeout", "3s", "-rotate-after", "100ms", "-retain-for", "200ms", "--", "/bin/sh", "-c", "printf 'terminal-fixture\\n'"})
	}()
	defer func() {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("terminal panel monitor did not stop")
		}
	}()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(2 * time.Second)
	sawRetained := false
	for time.Now().Before(deadline) {
		response, err := client.Get("http://" + address + "/status")
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 65537))
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatal(err)
		}
		var status struct {
			State        monitorState `json:"state"`
			SourceStatus string       `json:"source_status"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			t.Fatal(err)
		}
		if status.SourceStatus == "stopped" && status.State.SourceExit != nil && *status.State.SourceExit == 0 {
			if status.State.Logs.RetainedBytes > 0 {
				sawRetained = true
			}
			if sawRetained && status.State.Logs.RetainedBytes == 0 && status.State.Logs.ExpiredBytes > 0 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("logs outlived retain-for while terminal panel remained available")
}

func TestMonitorPublishesSourceExitBeforeSinkCleanup(t *testing.T) {
	out := filepath.Join(t.TempDir(), "monitor")
	root, err := openMonitorRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	at := time.Now().UTC()
	store, err := openLogStore(filepath.Join(out, "logs"), logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, at)
	if err != nil {
		t.Fatal(err)
	}
	consoleR, consoleW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer consoleR.Close()
	if err := consoleW.SetWriteDeadline(time.Now().Add(5 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	n, err := consoleW.Write(make([]byte, 1<<20))
	if n == 0 || err == nil {
		t.Fatalf("blocked pipe fixture invalid: n=%d err=%v", n, err)
	}
	delivery := newMonitorDelivery(store, consoleW, false, at)
	done := make(chan error, 1)
	go func() {
		done <- runMonitorSource(context.Background(), root, delivery, []string{"/bin/sh", "-c", "printf 'cleanup-fixture\\n'"})
	}()
	defer func() {
		select {
		case err := <-done:
			if err == nil {
				t.Error("blocked console lost failure")
			}
		case <-time.After(3 * time.Second):
			t.Error("cleanup did not join")
		}
	}()
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		state := delivery.snapshot()
		if state.SourceExit != nil {
			if state.SourceAlive || *state.SourceExit != 0 || state.SourceEnded == nil || !state.CleanupInProgress || state.Finished != nil {
				t.Fatalf("known source exit still presented as active: %+v", state)
			}
			response := httptest.NewRecorder()
			monitorHandler(delivery).ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:8094/status", nil))
			var projected struct {
				SourceStatus string `json:"source_status"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil || projected.SourceStatus != "stopped" {
				t.Fatalf("HTTP exit during cleanup: %s (%v)", response.Body.Bytes(), err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("source exit was held until sink cleanup")
}

func TestMonitorPeriodicSamplesDoNotDisplaceLogEvents(t *testing.T) {
	store, err := openLogStore(filepath.Join(t.TempDir(), "logs"), logRetentionPolicy{SegmentBytes: lineLimit, MaxBytes: 4 * lineLimit, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	delivery := newMonitorDelivery(store, nil, false, time.Now().UTC())
	delivery.row("stderr", []byte(`{"schema":"ardents-node-event-v1","kind":"lifecycle","state":"FAILED"}`), false)
	for i := 0; i < 300; i++ {
		delivery.row("stdout", []byte(`{"schema":"ardents-node-event-v1","kind":"resource-sample","state":"OBSERVED","resource":{"memory.current":1024}}`), false)
	}
	state := delivery.snapshot()
	if state.TotalRows != 1 || len(state.Tail) != 1 || state.Tail[0].Entry.State != "FAILED" || state.TailEvictedRows != 0 {
		t.Fatalf("periodic measurements displaced the failure: total=%d tail=%d evicted=%d", state.TotalRows, len(state.Tail), state.TailEvictedRows)
	}
	if state.TotalSamples != 300 || state.LatestSample == nil || state.LatestSample.Entry.Kind != "resource-sample" {
		t.Fatal("latest measurement or sample count missing")
	}
	if err := delivery.close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.root.Name())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-samples.log") {
			found = true
		}
	}
	if !found {
		t.Fatal("sample history was not routed to separate files")
	}
}

func TestMonitorMetricsScopesFreshnessAndDelivery(t *testing.T) {
	now := time.Now().UTC()
	state := monitorState{
		Schema: "ardents-monitor-v1", Started: now.Add(-time.Minute), Updated: now,
		SourceAlive: true, SourcePID: 987654, SourceName: "PRIVATE_SENTINEL",
		MetricSampleMaxAge: 5 * time.Second, SnapshotFailed: true,
		QueueDroppedBytes: 11, Logs: logStoreStats{LostBytes: 13, ExpiredBytes: 17},
		LatestSample: &monitorLogRow{At: now, Entry: event{
			Schema: "ardents-node-event-v1", Kind: "resource-sample", At: now.Format(time.RFC3339Nano),
			Resource: map[string]float64{"cpu_usage_usec": 2000000, "memory_bytes": 4096,
				"fds": 0, "go_memory_bytes": 0, "goroutines": 0, "socket_memory_bytes": 0,
				"cpu_pressure": 0, "memory_pressure": 0, "io_pressure": 0,
				"threads": 0, "sockets": 0, "high_events": 0, "emergency_events": 0,
				"rss_bytes": 0, "admission_active": 0, "queue_items": 0},
		}},
	}
	check := func(state monitorState) string {
		t.Helper()
		body, err := monitorMetrics(state, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > 16<<10 {
			t.Fatal("unbounded metric response")
		}
		return string(body)
	}
	body := check(state)
	for _, want := range []string{
		"diagnostic_selected_cgroup_cpu_usage_seconds_total 2\n",
		"diagnostic_selected_cgroup_memory_bytes 4096\n",
		"diagnostic_selected_snapshot_failed 1\n",
		"diagnostic_selected_queue_dropped_bytes_total 11\n",
		"diagnostic_selected_log_lost_bytes_total 13\n",
		"diagnostic_selected_log_expired_bytes_total 17\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, forbidden := range []string{"PRIVATE_SENTINEL", "987654", "rss_bytes", "admission", "queue_items", "process_fds", "process_go_memory", "process_goroutines", "socket_memory", "pressure_avg10", "process_threads", "process_sockets", "high_events", "emergency_events"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unsafe/unpopulated field %q", forbidden)
		}
	}
	state.LatestSample.Entry.At = now.Add(-10 * time.Second).Format(time.RFC3339Nano)
	if body = check(state); strings.Contains(body, "cgroup_memory_bytes") ||
		!strings.Contains(body, "diagnostic_selected_sample_fresh 0\n") {
		t.Fatal("recent receipt refreshed old producer sample")
	}
	state.Updated = now.Add(-4 * time.Second)
	if body = check(state); strings.Contains(body, "source_process_alive") ||
		strings.Contains(body, "snapshot_failed") ||
		!strings.Contains(body, "diagnostic_selected_monitor_fresh 0\n") {
		t.Fatal("stale supervisor presented current source/delivery health")
	}
	state.Updated = now
	state.SourceAlive = false
	if body = check(state); !strings.Contains(body, "diagnostic_selected_source_process_alive 0\n") ||
		strings.Contains(body, "cgroup_memory_bytes") {
		t.Fatal("stopped source exported resource values")
	}
}

func TestMonitorMetricsMemoryOnlyHTTPAndInvalidSource(t *testing.T) {
	now := time.Now().UTC()
	delivery := &monitorDelivery{state: monitorState{
		Schema: "ardents-monitor-v1", Started: now.Add(-time.Minute), Updated: now,
		SourceAlive: true, SourcePID: 1, FileFailed: true, SnapshotFailed: true,
	}}
	fetch := func(host string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		monitorHandler(delivery).ServeHTTP(response, httptest.NewRequest("GET", "http://"+host+"/metrics", nil))
		return response
	}
	// No log store, file queue, directory or snapshot writer exists in this
	// delivery. Its independent observed failures must still be readable.
	response := fetch("127.0.0.1:8094")
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "diagnostic_selected_log_file_failed 1\n") ||
		!strings.Contains(response.Body.String(), "diagnostic_selected_resource_export_enabled 0\n") {
		t.Fatal("memory-only metric endpoint unavailable during sink failure")
	}
	if response = fetch("foreign.example:8094"); response.Code != http.StatusForbidden {
		t.Fatal("metrics bypassed existing host protection")
	}
	delivery.state.Updated = now.Add(time.Hour)
	if response = fetch("127.0.0.1:8094"); response.Code != http.StatusServiceUnavailable ||
		strings.Contains(response.Body.String(), "diagnostic_selected_") {
		t.Fatal("invalid timestamps returned partial successful metrics")
	}
}

func TestCollectorMetricsTLSClientPinAndRestrictedSurface(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private diagnostic test CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, public, private)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, server bool, expired bool) (tls.Certificate, []byte, []byte) {
		t.Helper()
		keyPublic, keyPrivate, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		expiry := now.Add(time.Hour)
		if expired {
			expiry = now.Add(-time.Second)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "diagnostic test role"},
			NotBefore: now.Add(-time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		if server {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, keyPublic, private)
		if err != nil {
			t.Fatal(err)
		}
		encodedKey, err := x509.MarshalPKCS8PrivateKey(keyPrivate)
		if err != nil {
			t.Fatal(err)
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})
		certificate, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}
		return certificate, certPEM, keyPEM
	}
	_, serverCert, serverKey := issue(2, true, false)
	client, _, _ := issue(3, false, false)
	wrong, _, _ := issue(4, false, false)
	expired, _, _ := issue(5, false, true)
	clientLeaf, err := x509.ParseCertificate(client.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(clientLeaf.RawSubjectPublicKeyInfo)
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"server.crt": serverCert, "server.key": serverKey, "client-ca.crt": caPEM} {
		if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	delivery := &monitorDelivery{state: monitorState{
		Schema: "ardents-monitor-v1", Started: now.Add(-time.Minute), Updated: time.Now().UTC(),
		SourcePID: 1, SourceAlive: true,
	}}
	view, err := openCollectorMetrics("127.0.0.1:0", false, delivery, directory, hex.EncodeToString(pin[:]))
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			if err := view.close(); err != nil {
				t.Error(err)
			}
		}
	}()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("test CA invalid")
	}
	fetch := func(certificate *tls.Certificate, path string) (int, string, error) {
		configuration := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}
		if certificate != nil {
			configuration.Certificates = []tls.Certificate{*certificate}
		}
		transport := &http.Transport{TLSClientConfig: configuration}
		defer transport.CloseIdleConnections()
		connection := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		response, err := connection.Get("https://" + view.listener.Addr().String() + path)
		if err != nil {
			return 0, "", err
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		return response.StatusCode, string(body), errors.Join(readErr, closeErr)
	}
	for _, denied := range []*tls.Certificate{nil, &wrong, &expired} {
		if status, _, err := fetch(denied, "/metrics"); err == nil && status == http.StatusOK {
			t.Fatal("unselected or expired client obtained metrics")
		}
	}
	if status, body, err := fetch(&client, "/metrics"); err != nil || status != http.StatusOK ||
		!strings.Contains(body, "diagnostic_selected_source_process_alive 1\n") {
		t.Fatalf("selected diagnostic client failed: status=%d error=%v", status, err)
	}
	for _, path := range []string{"/", "/status", "/debug/pprof/heap", "/server.key"} {
		if status, _, err := fetch(&client, path); err != nil || status != http.StatusNotFound {
			t.Fatalf("collector gained unrelated surface %s: %d/%v", path, status, err)
		}
	}
	// Request-time certificate expiry remains a refusal on an already verified
	// connection; it does not depend on forcing another handshake.
	request := httptest.NewRequest("GET", "https://diagnostic.example/metrics", nil)
	expiredCopy := *clientLeaf
	expiredCopy.NotAfter = now.Add(-time.Second)
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{&expiredCopy},
		VerifiedChains: [][]*x509.Certificate{{&expiredCopy, ca}}}
	response := httptest.NewRecorder()
	view.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatal("retained expired TLS identity accepted")
	}
	if err := view.close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	select {
	case <-view.done:
	default:
		t.Fatal("collector listener did not join")
	}
	if connection, err := net.DialTimeout("tcp", view.listener.Addr().String(), time.Second); err == nil {
		connection.Close()
		t.Fatal("collector listener survived joined shutdown")
	}
	if err := os.Chmod(filepath.Join(directory, "server.key"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := openCollectorMetrics("127.0.0.1:0", false, delivery, directory, hex.EncodeToString(pin[:])); err == nil {
		t.Fatal("non-private diagnostic key accepted")
	}
}

func TestMonitorRetentionMetricsFollowRealRotationAndRestart(t *testing.T) {
	at := time.Now().UTC()
	directory := filepath.Join(t.TempDir(), "logs")
	policy := logRetentionPolicy{SegmentBytes: 4, MaxBytes: 8, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := openLogStore(directory, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"1111", "2222", "3333"} {
		if _, err := store.Append("events", []byte(body), at); err != nil {
			t.Fatal(err)
		}
	}
	check := func(store *logStore, now time.Time, expired string) {
		t.Helper()
		state := monitorState{Schema: "ardents-monitor-v1", Started: at.Add(-time.Minute), Updated: now, Limits: policy, Logs: store.Stats(), LogsObservedAt: now}
		body, err := monitorMetrics(state, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			"diagnostic_selected_log_storage_observation_available 1\n",
			"diagnostic_selected_log_filesystem_observation_available 1\n",
			"diagnostic_selected_log_retained_bytes 8\n",
			"diagnostic_selected_log_retained_files 3\n",
			"diagnostic_selected_log_retention_limit_bytes 8\n",
			"diagnostic_selected_log_retention_limit_files 3\n",
			"diagnostic_selected_log_expired_bytes_total " + expired + "\n",
			"diagnostic_selected_log_lost_bytes_total 0\n",
		} {
			if !strings.Contains(string(body), expected) {
				t.Fatalf("missing rotation observation %q: %s", expected, body)
			}
		}
		state.Updated = now.Add(-4 * time.Second)
		body, err = monitorMetrics(state, now)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "# HELP diagnostic_selected_log_retained_bytes") || strings.Contains(string(body), "# HELP diagnostic_selected_log_filesystem_total_bytes") || !strings.Contains(string(body), "diagnostic_selected_log_storage_observation_available 0\n") || !strings.Contains(string(body), "diagnostic_selected_log_filesystem_observation_available 0\n") {
			t.Fatal("stale payload accounting presented as current")
		}
		state.Updated = now // A fresh independent heartbeat cannot refresh blocked accounting.
		state.LogsObservedAt = now.Add(-4 * time.Second)
		body, err = monitorMetrics(state, now)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "# HELP diagnostic_selected_log_retained_bytes") || strings.Contains(string(body), "# HELP diagnostic_selected_log_filesystem_total_bytes") || !strings.Contains(string(body), "diagnostic_selected_log_storage_observation_available 0\n") || !strings.Contains(string(body), "diagnostic_selected_log_filesystem_observation_available 0\n") {
			t.Fatal("heartbeat refreshed stale file accounting")
		}
		state.LogsObservedAt = now
		state.Logs.RetainedBytes = policy.MaxBytes + 1
		if _, err := monitorMetrics(state, now); err == nil {
			t.Fatal("impossible accounting accepted")
		}
	}
	check(store, at, "4")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = openLogStore(directory, policy, at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	check(store, at.Add(time.Second), "0") // Retained inventory survives; session expiry counters reset.
}
