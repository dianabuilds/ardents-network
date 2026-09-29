//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const logLimit = 16 << 20
const recordLimit = 4 << 20
const lineLimit = 16 << 10

type boundedFile struct {
	file                    *os.File
	limit, written, dropped int64
	failure                 error
}

func (f *boundedFile) Write(body []byte) (int, error) {
	total := len(body)
	remaining := max(0, f.limit-f.written)
	if int64(len(body)) > remaining {
		f.dropped += int64(len(body)) - remaining
		body = body[:remaining]
	}
	if len(body) > 0 && f.failure == nil {
		n, err := f.file.Write(body)
		f.written += int64(n)
		if err != nil {
			f.failure = err
		} else if n != len(body) {
			f.failure = io.ErrShortWrite
		}
	}
	return total, nil // Drain even after saturation or sink failure; report it in summary.
}
func (f *boundedFile) Close() error {
	if f.file == nil {
		return f.failure
	}
	f.failure = errors.Join(f.failure, f.file.Close())
	f.file = nil
	return f.failure
}
func openBounded(dir, name string, limit int64) (*boundedFile, error) {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &boundedFile{file: f, limit: limit}, nil
}

type event struct {
	ObservedAt        time.Time          `json:"observed_at"`
	At                string             `json:"at,omitempty"`
	Schema            string             `json:"schema"`
	Kind              string             `json:"kind"`
	State             string             `json:"state,omitempty"`
	Carrier           string             `json:"carrier,omitempty"`
	Failure           string             `json:"failure,omitempty"`
	UnknownCategories uint64             `json:"unknown_categories,omitempty"`
	Resource          map[string]float64 `json:"resource,omitempty"`
	Hosting           map[string]float64 `json:"hosting,omitempty"`
}

type summary struct {
	Started            time.Time          `json:"started"`
	Finished           time.Time          `json:"finished,omitempty"`
	ExitCode           int                `json:"exit_code"`
	TimedOut           bool               `json:"timed_out"`
	Interrupted        bool               `json:"interrupted"`
	SourceChanged      bool               `json:"source_tree_changed"`
	CaptureIncomplete  bool               `json:"capture_incomplete"`
	StdoutBytes        int64              `json:"stdout_bytes"`
	StderrBytes        int64              `json:"stderr_bytes"`
	RawDroppedBytes    int64              `json:"raw_dropped_bytes"`
	EventDroppedBytes  int64              `json:"event_dropped_bytes"`
	SampleDroppedBytes int64              `json:"sample_dropped_bytes"`
	UnknownCategories  uint64             `json:"unknown_categories"`
	InvalidLines       uint64             `json:"invalid_lines"`
	UnrecognizedLines  uint64             `json:"unrecognized_lines"`
	OversizedLines     uint64             `json:"oversized_lines"`
	SampleFailures     uint64             `json:"sample_failures"`
	Samples            uint64             `json:"samples"`
	Counts             map[string]uint64  `json:"counts"`
	LastResource       *event             `json:"last_resource,omitempty"`
	LastEvents         []event            `json:"last_events"`
	Peak               map[string]float64 `json:"peak"`
}

type projection struct {
	mu      sync.Mutex
	summary summary
	events  *boundedFile
}

var resourceFields = strings.Fields("rss_bytes cpu_usage_usec memory_bytes go_memory_bytes socket_memory_bytes sockets fds goroutines threads timers queue_items queue_bytes storage_bytes storage_files cpu_pressure memory_pressure io_pressure high_events emergency_events admission_active admission_accepted admission_refused")
var hostingFields = strings.Fields("UsedBytes ReservedBytes RemainingBytes")

func numbers(value any, fields []string) map[string]float64 {
	object, _ := value.(map[string]any)
	result := map[string]float64{}
	for _, field := range fields {
		n, ok := object[field].(float64)
		if ok && n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			result[field] = n
		}
	}
	return result
}
func selected(value any, allowed string) string {
	text, _ := value.(string)
	for _, item := range strings.Fields(allowed) {
		if text == item {
			return text
		}
	}
	return ""
}
func project(body []byte) (event, string) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return event{}, "invalid"
	}
	if message, ok := data["MESSAGE"].(string); ok {
		if err := json.Unmarshal([]byte(message), &data); err != nil {
			return event{}, "invalid"
		}
	}
	schema := selected(data["schema"], "ardents-node-event-v1 ardents-headless-runtime-event-v1 ardents-source-event-v1")
	if schema == "" {
		return event{}, "unknown"
	}
	kinds := "lifecycle resource resource-sample"
	if schema == "ardents-source-event-v1" {
		kinds = "source-ready source-wave-accepted source-failed"
	}
	if schema == "ardents-headless-runtime-event-v1" {
		kinds = "headless-runtime-ready headless-runtime-failed headless-runtime-permission-required headless-runtime-publication-refresh-failed headless-runtime-publication-withdrawal-failed headless-runtime-connection-operation-failed"
	}
	kind := selected(data["kind"], kinds)
	if kind == "" {
		return event{}, "unknown"
	}
	at := ""
	if text, ok := data["at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			at = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	e := event{ObservedAt: time.Now().UTC(), At: at, Schema: schema, Kind: kind,
		State:   selected(data["state"], "ABSENT PREPARED READY DRAINING WITHDRAWN FAILED OBSERVED NORMAL PROTECT DRAIN EXIT"),
		Carrier: selected(data["carrier_profile"], "ardents-carrier-tcp-tls-v2 ardents-carrier-quic-v2"),
		Failure: selected(data["failure"], "startup running rotation authorization publication-state publication-handover caller-context publisher-ended publication-draining registration-absent publisher-not-live registration publisher-drain deadline admission activation worker-launch worker-operation introduction-preparation service-join post-join-lifetime service-result rotation-authority rotation-prefix rotation-recipient rotation-expired rotation-registration rotation-publication"), Resource: numbers(data["resource"], resourceFields)}
	if schema == "ardents-source-event-v1" {
		e.Failure = selected(data["reason"], "background-work cleanup")
	}
	if schema == "ardents-node-event-v1" {
		e.Failure = ""
	}
	for key, value := range map[string]string{"state": e.State, "carrier_profile": e.Carrier} {
		if raw, ok := data[key].(string); ok && raw != "" && value == "" {
			e.UnknownCategories++
		}
	}
	failureKey := "failure"
	if schema == "ardents-source-event-v1" {
		failureKey = "reason"
	}
	if schema != "ardents-node-event-v1" {
		if raw, ok := data[failureKey].(string); ok && raw != "" && e.Failure == "" {
			e.Failure = "unclassified"
			e.UnknownCategories++
		}
	}
	if hosting, ok := data["hosting"].(map[string]any); ok {
		e.Hosting = numbers(hosting["Observation"], hostingFields)
	}
	return e, ""
}
func (p *projection) line(body []byte) {
	// go test -json wraps product output. Unwrap only its string Output, never arbitrary fields.
	var wrapper struct {
		Action string
		Output string
	}
	if json.Unmarshal(body, &wrapper) == nil && wrapper.Action == "output" {
		body = []byte(strings.TrimSpace(wrapper.Output))
	}
	e, reason := project(body)
	p.mu.Lock()
	defer p.mu.Unlock()
	if reason == "invalid" {
		p.summary.InvalidLines++
		return
	}
	if reason != "" {
		p.summary.UnrecognizedLines++
		return
	}
	p.summary.Counts[e.Kind]++
	p.summary.UnknownCategories += e.UnknownCategories
	p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || e.UnknownCategories > 0
	if e.Kind == "resource-sample" {
		p.summary.LastResource = &e
	} else {
		p.summary.LastEvents = append(p.summary.LastEvents, e)
		if len(p.summary.LastEvents) > 256 {
			p.summary.LastEvents = p.summary.LastEvents[1:]
		}
	}
	raw, err := json.Marshal(e)
	if err == nil {
		p.events.Write(append(raw, '\n'))
		p.summary.EventDroppedBytes = p.events.dropped
		p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || p.events.dropped > 0 || p.events.failure != nil
	}
	for key, value := range e.Resource {
		if value > p.summary.Peak[key] {
			p.summary.Peak[key] = value
		}
	}
}
func drain(reader io.Reader, raw *boundedFile, p *projection, stdout bool) error {
	buffer := bufio.NewReaderSize(reader, lineLimit)
	line := make([]byte, 0, lineLimit)
	oversized := false
	for {
		fragment, err := buffer.ReadSlice('\n')
		if raw != nil {
			before := raw.dropped
			raw.Write(fragment)
			p.mu.Lock()
			p.summary.RawDroppedBytes += raw.dropped - before
			p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || raw.failure != nil || raw.dropped > 0
			p.mu.Unlock()
		}
		p.mu.Lock()
		if stdout {
			p.summary.StdoutBytes += int64(len(fragment))
		} else {
			p.summary.StderrBytes += int64(len(fragment))
		}
		p.mu.Unlock()
		if stdout {
			if len(line)+len(fragment) > lineLimit {
				oversized = true
				line = line[:0]
			}
			if !oversized {
				line = append(line, fragment...)
			}
			if err != bufio.ErrBufferFull {
				if oversized {
					p.mu.Lock()
					p.summary.OversizedLines++
					p.mu.Unlock()
				} else if len(bytes.TrimSpace(line)) > 0 {
					p.line(bytes.TrimSpace(line))
				}
				line = line[:0]
				oversized = false
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil && err != bufio.ErrBufferFull {
			return err
		}
	}
}

func supervise(dir, source string, command []string, timeout time.Duration, raw bool) (outcome error) {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, timeout)
	defer cancel()
	p := &projection{summary: summary{Started: time.Now().UTC(), ExitCode: -1, Counts: map[string]uint64{}, Peak: map[string]float64{}}}
	terminalWritten := false
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	defer func() {
		if !terminalWritten {
			p.summary.Finished = time.Now().UTC()
			p.summary.CaptureIncomplete = true
			p.summary.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
			p.summary.Interrupted = ctx.Err() != nil && !p.summary.TimedOut
			outcome = errors.Join(outcome, writeJSON(filepath.Join(dir, "summary.json"), p.summary))
		}
	}()
	info := map[string]any{"schema": "ardents-local-diagnostics-v1", "started": p.summary.Started, "timeout": timeout.String(), "raw": raw, "command": filepath.Base(command[0]), "arguments_retained": raw, "source_sha": sourceRevision(ctx, source), "source_tree_sha256": sourceDiffDigest(ctx, source), "go": query(ctx, source, "go", "version"), "image": os.Getenv("ARDENTS_DIAGNOSTIC_IMAGE"), "scope": "local development; process-group samples and network-namespace counters, not installed qualification"}
	if inventory, err := os.ReadFile("/opt/ardents-diagnostics/inventory.txt"); err == nil && len(inventory) <= 1<<20 {
		if err := os.WriteFile(filepath.Join(dir, "tools.txt"), inventory, 0600); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(dir, "manifest.json"), info); err != nil {
		return err
	}
	if info["source_tree_sha256"] == "unavailable" {
		return errors.Join(ctx.Err(), errors.New("source inventory unavailable"))
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if raw {
		if err := writeJSON(filepath.Join(dir, "command.json"), command); err != nil {
			return err
		}
	}
	events, err := openBounded(dir, "events.ndjson", recordLimit)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, events.Close()) }()
	samples, err := openBounded(dir, "samples.ndjson", recordLimit)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, samples.Close()) }()
	p.events = events
	var stdoutRaw, stderrRaw *boundedFile
	if raw {
		stdoutRaw, err = openBounded(dir, "stdout.log", logLimit)
		if err != nil {
			return err
		}
		defer func() { outcome = errors.Join(outcome, stdoutRaw.Close()) }()
		stderrRaw, err = openBounded(dir, "stderr.log", logLimit)
		if err != nil {
			return err
		}
		defer func() { outcome = errors.Join(outcome, stderrRaw.Close()) }()
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = source
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stdoutR.Close()
	defer stdoutW.Close()
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stderrR.Close()
	defer stderrW.Close()
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	startErr := ctx.Err()
	if startErr == nil {
		startErr = cmd.Start()
	}
	stdoutW.Close()
	stderrW.Close()
	if startErr != nil {
		p.summary.Finished = time.Now().UTC()
		p.summary.CaptureIncomplete = true
		terminalWritten = true
		return errors.Join(startErr, writeJSON(filepath.Join(dir, "summary.json"), p.summary))
	}
	drains := make(chan error, 2)
	go func() { drains <- drain(stdoutR, stdoutRaw, p, true) }()
	go func() { drains <- drain(stderrR, stderrRaw, p, false) }()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var commandErr error
	waiting := true
	for waiting {
		select {
		case commandErr = <-done:
			waiting = false
		case <-ctx.Done():
			p.mu.Lock()
			p.summary.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
			p.summary.Interrupted = !p.summary.TimedOut
			p.mu.Unlock()
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			timer := time.NewTimer(3 * time.Second)
			select {
			case commandErr = <-done:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				commandErr = <-done
			}
			waiting = false
		case at := <-tick.C:
			if err := p.snapshot(dir); err != nil {
				p.mu.Lock()
				p.summary.CaptureIncomplete = true
				p.mu.Unlock()
			}
			sample, sampleErr := sampleGroup(cmd.Process.Pid, at)
			p.mu.Lock()
			if sampleErr != nil {
				p.summary.SampleFailures++
			} else {
				p.summary.Samples++
			}
			p.mu.Unlock()
			if sampleErr == nil {
				body, _ := json.Marshal(sample)
				samples.Write(append(body, '\n'))
				p.mu.Lock()
				p.summary.SampleDroppedBytes = samples.dropped
				p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || samples.dropped > 0 || samples.failure != nil
				p.mu.Unlock()
			}
		}
	}
	// A child may exit while descendants still hold pipes; kill the owned group and bound draining.
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	var drainErr error
	for range 2 {
		select {
		case err := <-drains:
			drainErr = errors.Join(drainErr, err)
		case <-time.After(3 * time.Second):
			stdoutR.Close()
			stderrR.Close()
			drainErr = errors.Join(drainErr, errors.New("output drain timed out"))
			err := <-drains
			drainErr = errors.Join(drainErr, err)
		}
	}
	if ctx.Err() == nil {
		finalHash := sourceDiffDigest(ctx, source)
		p.summary.SourceChanged = finalHash == "unavailable" || finalHash != info["source_tree_sha256"]
		p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || p.summary.SourceChanged
	}
	if ctx.Err() != nil {
		p.summary.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		p.summary.Interrupted = !p.summary.TimedOut
	}
	p.summary.Finished = time.Now().UTC()
	p.summary.ExitCode = cmd.ProcessState.ExitCode()
	if stdoutRaw != nil {
		p.summary.RawDroppedBytes = stdoutRaw.dropped + stderrRaw.dropped
	}
	p.summary.EventDroppedBytes = events.dropped
	p.summary.SampleDroppedBytes = samples.dropped
	p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || drainErr != nil || events.failure != nil || samples.failure != nil || p.summary.RawDroppedBytes > 0 || events.dropped > 0 || samples.dropped > 0 || p.summary.OversizedLines > 0 || p.summary.SampleFailures > 0
	if stdoutRaw != nil {
		p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || stdoutRaw.failure != nil || stderrRaw.failure != nil
	}
	closeErr := errors.Join(events.Close(), samples.Close())
	if stdoutRaw != nil {
		closeErr = errors.Join(closeErr, stdoutRaw.Close(), stderrRaw.Close())
	}
	p.summary.CaptureIncomplete = p.summary.CaptureIncomplete || closeErr != nil
	terminalWritten = true
	summaryErr := errors.Join(closeErr, writeJSON(filepath.Join(dir, "summary.json"), p.summary))
	fmt.Printf("evidence: %s; exit=%d timeout=%t incomplete=%t\n", dir, p.summary.ExitCode, p.summary.TimedOut, p.summary.CaptureIncomplete)
	if p.summary.TimedOut || p.summary.Interrupted || p.summary.CaptureIncomplete {
		return errors.Join(commandErr, drainErr, summaryErr, errors.New("diagnostic run failed, interrupted, or incomplete"))
	}
	return errors.Join(commandErr, drainErr, summaryErr)
}

func query(parent context.Context, dir, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	body, err := cmd.Output()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(body))
}

type processSample struct {
	At          time.Time          `json:"at"`
	Processes   uint64             `json:"processes"`
	RSSBytes    uint64             `json:"rss_bytes"`
	Threads     uint64             `json:"threads"`
	FDs         uint64             `json:"fds"`
	CPUJiffies  uint64             `json:"cpu_jiffies"`
	Network     map[string]uint64  `json:"network_namespace"`
	Cgroup      map[string]uint64  `json:"container_cgroup,omitempty"`
	Pressure    map[string]float64 `json:"container_pressure_avg10,omitempty"`
	Unavailable []string           `json:"unavailable,omitempty"`
}

func sampleGroup(group int, at time.Time) (processSample, error) {
	s := processSample{At: at.UTC(), Network: map[string]uint64{}}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return s, err
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		root := filepath.Join("/proc", entry.Name())
		body, err := os.ReadFile(filepath.Join(root, "stat"))
		if err != nil {
			continue
		}
		end := bytes.LastIndexByte(body, ')')
		if end < 0 {
			continue
		}
		fields := strings.Fields(string(body[end+1:]))
		if len(fields) < 22 {
			continue
		}
		pgid, _ := strconv.Atoi(fields[2])
		if pgid != group {
			continue
		}
		if s.Processes >= 1024 {
			return s, errors.New("process observation bound exceeded")
		}
		s.Processes++
		user, _ := strconv.ParseUint(fields[11], 10, 64)
		system, _ := strconv.ParseUint(fields[12], 10, 64)
		s.CPUJiffies += user + system
		threads, _ := strconv.ParseUint(fields[17], 10, 64)
		s.Threads += threads
		pages, _ := strconv.ParseUint(fields[21], 10, 64)
		s.RSSBytes += pages * uint64(os.Getpagesize())
		fds, err := os.ReadDir(filepath.Join(root, "fd"))
		if err == nil {
			s.FDs += uint64(len(fds))
		} else if len(s.Unavailable) == 0 {
			s.Unavailable = append(s.Unavailable, "some_process_fds")
		}
	}
	if s.Processes == 0 {
		return s, errors.New("process group unavailable at sample")
	}
	snmp, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return s, err
	}
	lines := strings.Split(strings.TrimSpace(string(snmp)), "\n")
	for i := 0; i+1 < len(lines); i += 2 {
		keys := strings.Fields(lines[i])
		values := strings.Fields(lines[i+1])
		if len(keys) != len(values) {
			continue
		}
		for j := 1; j < len(keys); j++ {
			key := keys[0] + keys[j]
			if selected(key, "Tcp:RetransSegs Tcp:InErrs Tcp:OutRsts Tcp:CurrEstab Udp:InErrors Udp:RcvbufErrors Udp:SndbufErrors") != "" {
				s.Network[key], _ = strconv.ParseUint(values[j], 10, 64)
			}
		}
	}
	s.Cgroup = map[string]uint64{}
	s.Pressure = map[string]float64{}
	for _, name := range []string{"memory.current", "pids.current"} {
		body, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", name))
		if err != nil {
			s.Unavailable = append(s.Unavailable, name)
			continue
		}
		value, err := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 64)
		if err == nil {
			s.Cgroup[name] = value
		} else {
			s.Unavailable = append(s.Unavailable, name)
		}
	}
	for _, name := range []string{"cpu.stat", "memory.events"} {
		body, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", name))
		if err != nil {
			s.Unavailable = append(s.Unavailable, name)
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			if selected(fields[0], "usage_usec throttled_usec nr_throttled high oom oom_kill") != "" {
				value, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					s.Cgroup[name+":"+fields[0]] = value
				}
			}
		}
	}
	for _, name := range []string{"cpu", "memory", "io"} {
		body, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", name+".pressure"))
		if err != nil {
			s.Unavailable = append(s.Unavailable, name+".pressure")
			continue
		}
		fields := strings.Fields(strings.SplitN(string(body), "\n", 2)[0])
		for _, field := range fields {
			if strings.HasPrefix(field, "avg10=") {
				value, err := strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
				if err == nil && value >= 0 && !math.IsInf(value, 0) && !math.IsNaN(value) {
					s.Pressure[name] = value
				}
			}
		}
	}
	return s, nil
}

func sourceRevision(ctx context.Context, source string) string {
	revision := os.Getenv("ARDENTS_DIAGNOSTIC_SOURCE_SHA")
	if len(revision) == 40 && strings.IndexFunc(revision, func(c rune) bool { return !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') }) < 0 {
		return revision
	}
	return query(ctx, source, "git", "rev-parse", "HEAD")
}
