//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
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
	body, err := json.Marshal(map[string]any{"schema": "ardents-node-event-v1", "kind": "resource-sample", "state": "OBSERVED", "resource": values, "hosting": map[string]any{"Observation": map[string]float64{"UsedBytes": math.MaxFloat64, "ReservedBytes": math.MaxFloat64, "RemainingBytes": math.MaxFloat64}}})
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
