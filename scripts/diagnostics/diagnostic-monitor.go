//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const monitorTailBytes = 48 << 10

type monitorLogRow struct {
	Sequence uint64    `json:"sequence"`
	At       time.Time `json:"at"`
	Stream   string    `json:"stream"`
	Entry    event     `json:"entry"`
}
type monitorState struct {
	MetricsFailed       bool               `json:"metrics_failed"`
	MetricSampleMaxAge  time.Duration      `json:"metric_sample_max_age_ns"`
	SourceName          string             `json:"source_name"`
	SourcePID           int                `json:"source_pid"`
	LastOutput          *time.Time         `json:"last_output,omitempty"`
	TailEvictedRows     uint64             `json:"tail_evicted_rows"`
	TotalSamples        uint64             `json:"total_samples"`
	LatestSample        *monitorLogRow     `json:"latest_sample,omitempty"`
	TotalRows           uint64             `json:"total_rows"`
	Limits              logRetentionPolicy `json:"limits"`
	Schema              string             `json:"schema"`
	Started             time.Time          `json:"started"`
	Updated             time.Time          `json:"updated"`
	Finished            *time.Time         `json:"finished,omitempty"`
	StartFailed         bool               `json:"start_failed"`
	SourceAlive         bool               `json:"source_alive"`
	SourceEnded         *time.Time         `json:"source_ended,omitempty"`
	RetentionActive     bool               `json:"retention_active"`
	RetentionFailed     bool               `json:"retention_failed"`
	CleanupInProgress   bool               `json:"cleanup_in_progress"`
	SourceExit          *int               `json:"source_exit,omitempty"`
	Interrupted         bool               `json:"interrupted"`
	TimedOut            bool               `json:"timed_out"`
	Raw                 bool               `json:"raw"`
	StdoutBytes         int64              `json:"stdout_bytes"`
	StderrBytes         int64              `json:"stderr_bytes"`
	QueueDroppedBytes   int64              `json:"queue_dropped_bytes"`
	ConsoleDroppedBytes int64              `json:"console_dropped_bytes"`
	UnknownLines        uint64             `json:"unknown_lines"`
	OversizedLines      uint64             `json:"oversized_lines"`
	FileFailed          bool               `json:"file_failed"`
	ConsoleFailed       bool               `json:"console_failed"`
	PanelFailed         bool               `json:"panel_failed"`
	SnapshotFailed      bool               `json:"snapshot_failed"`
	SinksJoined         bool               `json:"sinks_joined"`
	CleanupFailed       bool               `json:"cleanup_failed"`
	Logs                logStoreStats      `json:"logs"`
	Tail                []monitorLogRow    `json:"tail"`
}
type monitorRecord struct {
	stream string
	body   []byte
	at     time.Time
}
type monitorDelivery struct {
	mu                    sync.Mutex
	state                 monitorState
	tailSizes             []int
	tailBytes             int
	fileQueue             chan monitorRecord
	consoleQueue          chan []byte
	fileDone, consoleDone chan error
	store                 *logStore
	console               *os.File
}

// File I/O never runs in producer drains or while holding the state lock.
func newMonitorDelivery(store *logStore, console *os.File, raw bool, at time.Time) *monitorDelivery {
	d := &monitorDelivery{store: store, console: console, fileQueue: make(chan monitorRecord, 128), consoleQueue: make(chan []byte, 64), fileDone: make(chan error, 1), consoleDone: make(chan error, 1),
		state: monitorState{Schema: "ardents-monitor-v1", Started: at, Updated: at, Raw: raw, SourceName: "selected-process", Limits: store.policy, Logs: store.Stats()}}
	go d.runFile()
	go d.runConsole()
	return d
}
func (d *monitorDelivery) snapshot() monitorState {
	d.mu.Lock()
	defer d.mu.Unlock()
	copy := d.state
	copy.Tail = append([]monitorLogRow(nil), d.state.Tail...)
	return copy
}
func (d *monitorDelivery) enqueue(stream string, body []byte, at time.Time) {
	record := monitorRecord{stream: stream, body: append([]byte(nil), body...), at: at}
	select {
	case d.fileQueue <- record:
	default:
		d.mu.Lock()
		d.state.QueueDroppedBytes += int64(len(body))
		d.mu.Unlock()
	}
}
func (d *monitorDelivery) row(stream string, body []byte, oversized bool) {
	at := time.Now().UTC()
	entry, reason := project(bytes.TrimSpace(body))
	if reason != "" || oversized {
		entry = event{Schema: "ardents-monitor-v1", Kind: "unstructured-output", ObservedAt: at}
		d.mu.Lock()
		if oversized {
			d.state.OversizedLines++
		} else {
			d.state.UnknownLines++
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	isSample := entry.Kind == "resource-sample"
	var sequence uint64
	if isSample {
		d.state.TotalSamples++
		sequence = d.state.TotalSamples
	} else {
		d.state.TotalRows++
		sequence = d.state.TotalRows
	}
	row := monitorLogRow{Sequence: sequence, At: at, Stream: stream, Entry: entry}
	encoded, err := json.Marshal(row)
	if err != nil {
		d.state.CleanupFailed = true
		d.mu.Unlock()
		return
	}
	encoded = append(encoded, '\n')
	if isSample {
		d.state.LatestSample = &row
		d.state.Updated = at
		d.mu.Unlock()
		d.enqueue("samples", encoded, at)
		return
	}
	d.state.Tail = append(d.state.Tail, row)
	d.tailSizes = append(d.tailSizes, len(encoded))
	d.tailBytes += len(encoded)
	for len(d.state.Tail) > 64 || d.tailBytes > monitorTailBytes {
		d.tailBytes -= d.tailSizes[0]
		d.tailSizes = d.tailSizes[1:]
		d.state.Tail = d.state.Tail[1:]
		d.state.TailEvictedRows++
	}
	d.state.Updated = at
	d.mu.Unlock()
	d.enqueue("events", encoded, at)
	if d.console != nil {
		select {
		case d.consoleQueue <- encoded:
		default:
			d.mu.Lock()
			d.state.ConsoleDroppedBytes += int64(len(encoded))
			d.mu.Unlock()
		}
	}
}
func (d *monitorDelivery) drain(reader io.Reader, stream string) error {
	buffer := bufio.NewReaderSize(reader, lineLimit)
	line := make([]byte, 0, lineLimit)
	oversized := false
	for {
		fragment, err := buffer.ReadSlice('\n')
		d.mu.Lock()
		if stream == "stdout" {
			d.state.StdoutBytes += int64(len(fragment))
		} else {
			d.state.StderrBytes += int64(len(fragment))
		}
		raw := d.state.Raw
		if len(fragment) > 0 {
			at := time.Now().UTC()
			d.state.LastOutput = &at
		}
		d.mu.Unlock()
		if raw && len(fragment) > 0 {
			d.enqueue(stream, fragment, time.Now().UTC())
		}
		if len(line)+len(fragment) > lineLimit {
			oversized = true
			line = line[:0]
		}
		if !oversized {
			line = append(line, fragment...)
		}
		if err != bufio.ErrBufferFull {
			if len(line) > 0 || oversized {
				d.row(stream, line, oversized)
			}
			line = line[:0]
			oversized = false
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil && err != bufio.ErrBufferFull {
			return err
		}
	}
}
func (d *monitorDelivery) runFile() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case record, ok := <-d.fileQueue:
			if !ok {
				err := d.store.Close()
				d.mu.Lock()
				d.state.Logs = d.store.Stats()
				d.state.FileFailed = d.state.FileFailed || err != nil
				d.mu.Unlock()
				d.fileDone <- err
				return
			}
			_, err := d.store.Append(record.stream, record.body, record.at)
			stats := d.store.Stats()
			d.mu.Lock()
			d.state.Logs = stats
			d.state.FileFailed = d.state.FileFailed || err != nil
			d.mu.Unlock()
		case at := <-tick.C:
			err := d.store.Prune(at.UTC())
			stats := d.store.Stats()
			d.mu.Lock()
			d.state.Logs = stats
			d.state.FileFailed = d.state.FileFailed || err != nil
			d.mu.Unlock()
		}
	}
}
func (d *monitorDelivery) runConsole() {
	var outcome error
	for body := range d.consoleQueue {
		if d.console == nil {
			continue
		}
		err := outcome
		n := 0
		if err == nil {
			err = d.console.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
			if err == nil {
				n, err = d.console.Write(body)
			}
		}
		if err == nil && n != len(body) {
			err = io.ErrShortWrite
		}
		d.mu.Lock()
		d.state.ConsoleDroppedBytes += int64(len(body) - n)
		d.state.ConsoleFailed = d.state.ConsoleFailed || err != nil
		d.mu.Unlock()
		if outcome == nil {
			outcome = err
		}
	}
	if d.console != nil {
		outcome = errors.Join(outcome, d.console.Close())
	}
	d.consoleDone <- outcome
}
func (d *monitorDelivery) close() error {
	close(d.fileQueue)
	close(d.consoleQueue)
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	var outcome error
	fileDone, consoleDone := d.fileDone, d.consoleDone
	for fileDone != nil || consoleDone != nil {
		select {
		case err := <-fileDone:
			outcome = errors.Join(outcome, err)
			fileDone = nil
		case err := <-consoleDone:
			outcome = errors.Join(outcome, err)
			consoleDone = nil
		case <-deadline.C:
			d.mu.Lock()
			d.state.CleanupFailed = true
			d.mu.Unlock()
			return errors.Join(outcome, errors.New("monitor sink join exceeded bound"))
		}
	}
	d.mu.Lock()
	d.state.SinksJoined = true
	d.mu.Unlock()
	return outcome
}
func openMonitorConsole(enabled bool) (*os.File, error) {
	if !enabled {
		return nil, nil
	}
	out, err := os.OpenFile("/proc/self/fd/1", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	if err := out.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, errors.Join(errors.New("console must support bounded writes; use -console=false for redirected files"), out.Close())
	}
	return out, nil
}
func openMonitorRoot(dir string) (_ *os.Root, outcome error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, errors.New("monitor out must be canonical absolute path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil || parent != filepath.Dir(dir) {
		return nil, errors.New("monitor parent must be canonical")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	source, err := filepath.EvalSymlinks(cwd)
	if err != nil || within(source, dir) {
		return nil, errors.New("monitor evidence must be outside source")
	}
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil || real != dir {
		return nil, errors.New("monitor directory must be canonical")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm() != 0700 || owner.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("monitor directory must be owner-private")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	admitted := false
	defer func() {
		if !admitted {
			outcome = errors.Join(outcome, root.Close())
		}
	}()
	entries, err := readLogInventory(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "logs":
			info, err := root.Lstat("logs")
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, errors.New("unsafe log directory")
			}
		case "monitor.json":
			file, err := root.OpenFile("monitor.json", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if err != nil {
				return nil, err
			}
			info, statErr := file.Stat()
			body, readErr := io.ReadAll(io.LimitReader(file, 65537))
			closeErr := file.Close()
			if statErr != nil || !privateLogFile(info) || len(body) > 65536 || !validJSONRecord(body) {
				return nil, errors.Join(statErr, readErr, closeErr, errors.New("invalid monitor snapshot"))
			}
			var previous monitorState
			if err := json.Unmarshal(body, &previous); err != nil || previous.Schema != "ardents-monitor-v1" {
				return nil, errors.New("unrecognized monitor snapshot")
			}
			if err := errors.Join(readErr, closeErr); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("unrecognized file in monitor directory")
		}
	}
	admitted = true
	return root, nil
}
func writeMonitorState(root *os.Root, state monitorState) (outcome error) {
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(body) > 65536 {
		return errors.New("monitor snapshot exceeds bound")
	}
	file, err := root.OpenFile("monitor.json.tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(body)
	if writeErr == nil && n != len(body) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return root.Rename("monitor.json.tmp", "monitor.json")
}

func monitorCommand(args []string) (outcome error) {
	flags := flag.NewFlagSet("monitor", flag.ContinueOnError)
	out := flags.String("out", "", "private retained monitoring directory outside source")
	raw := flags.Bool("raw", false, "retain sensitive stdout and stderr")
	console := flags.Bool("console", true, "live projected JSON console (bounded pipe/terminal)")
	listen := flags.String("listen", "", "optional local live log panel address")
	metricsListen := flags.String("metrics-listen", "", "optional separate mutual-TLS collector metrics address")
	metricsCerts := flags.String("metrics-certs", "", "private diagnostic directory: server.crt, server.key, client-ca.crt")
	metricsPin := flags.String("metrics-client-pin", "", "SHA256 of the selected diagnostic client's DER SubjectPublicKeyInfo")
	container := flags.Bool("container", false, "allow container wildcard; publish host loopback only")
	name := flags.String("name", "selected-process", "safe local source label (ASCII letters, digits, dash, underscore)")
	timeout := flags.Duration("timeout", 0, "optional finite run budget; zero follows source until signal")
	sampleMaxAge := flags.Duration("sample-max-age", 0, "explicit Node metric freshness budget; zero disables resource export")
	segmentBytes := flags.Int64("segment-bytes", 8<<20, "bytes per log segment")
	maxBytes := flags.Int64("retain-bytes", 64<<20, "total log bytes")
	maxFiles := flags.Int("retain-files", 9, "log files including empty ownership lock")
	segmentAge := flags.Duration("rotate-after", 15*time.Minute, "rotate on next record after age")
	maxAge := flags.Duration("retain-for", 24*time.Hour, "maximum segment age")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !validMonitorName(*name) {
		return errors.New("monitor name must be a safe label of at most 64 ASCII characters")
	}
	if len(flags.Args()) == 0 || *timeout < 0 || *timeout > 24*time.Hour || *segmentBytes < lineLimit || *sampleMaxAge < 0 || *sampleMaxAge > time.Hour {
		return errors.New("monitor requires explicit -- command and valid limits")
	}
	if (*metricsListen == "") != (*metricsCerts == "") || (*metricsListen == "") != (*metricsPin == "") {
		return errors.New("collector metrics requires address, private certificates and one client pin")
	}
	root, err := openMonitorRoot(*out)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	store, err := openLogStore(filepath.Join(*out, "logs"), logRetentionPolicy{SegmentBytes: *segmentBytes, MaxBytes: *maxBytes, MaxFiles: *maxFiles, SegmentAge: *segmentAge, MaxAge: *maxAge}, time.Now().UTC())
	if err != nil {
		return err
	}
	output, err := openMonitorConsole(*console)
	if err != nil {
		return errors.Join(err, store.Close())
	}
	delivery := newMonitorDelivery(store, output, *raw, time.Now().UTC())
	delivery.mu.Lock()
	delivery.state.SourceName = *name
	delivery.state.MetricSampleMaxAge = *sampleMaxAge
	delivery.mu.Unlock()
	var view *monitorView
	if *listen != "" {
		view, err = openMonitorView(*listen, *container, delivery)
		if err != nil {
			return errors.Join(err, delivery.close())
		}
		defer func() { outcome = errors.Join(outcome, view.close()) }()
	}
	var collector *monitorView
	if *metricsListen != "" {
		collector, err = openCollectorMetrics(*metricsListen, *container, delivery, *metricsCerts, *metricsPin)
		if err != nil {
			return errors.Join(err, delivery.close())
		}
		defer func() { outcome = errors.Join(outcome, collector.close()) }()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	sourceErr := runMonitorSource(ctx, root, delivery, flags.Args())
	if (view != nil || collector != nil) && ctx.Err() == nil {
		done := collector
		if view != nil {
			done = view
		}
		sourceErr = errors.Join(sourceErr, maintainMonitorLogs(ctx, root, delivery, done.done))
	}
	return sourceErr
}
func runMonitorSource(ctx context.Context, root *os.Root, delivery *monitorDelivery, command []string) (outcome error) {
	snapshots := newMonitorSnapshots(root, delivery)
	closed := false
	defer func() {
		if !closed {
			sinkErr := delivery.close()
			finished := time.Now().UTC()
			delivery.mu.Lock()
			delivery.state.Finished = &finished
			delivery.state.Updated = finished
			delivery.state.StartFailed = ctx.Err() == nil && outcome != nil
			delivery.state.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
			delivery.state.Interrupted = ctx.Err() != nil && !delivery.state.TimedOut
			delivery.state.CleanupFailed = delivery.state.CleanupFailed || sinkErr != nil
			delivery.mu.Unlock()
			outcome = errors.Join(outcome, sinkErr)
		}
		snapshots.offer(delivery.snapshot())
		outcome = errors.Join(outcome, snapshots.close())
		state := delivery.snapshot()
		if state.FileFailed || state.ConsoleFailed || state.SnapshotFailed || state.CleanupFailed || state.QueueDroppedBytes > 0 || state.ConsoleDroppedBytes > 0 || state.Logs.LostBytes > 0 {
			outcome = errors.Join(outcome, errors.New("monitoring failed or lost evidence"))
		}
	}()
	cmd := exec.Command(command[0], command[1:]...)
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	stdoutW.Close()
	stderrW.Close()
	delivery.mu.Lock()
	delivery.state.SourceAlive = true
	delivery.state.SourcePID = cmd.Process.Pid
	delivery.mu.Unlock()
	drains := make(chan error, 2)
	go func() { drains <- delivery.drain(stdoutR, "stdout") }()
	go func() { drains <- delivery.drain(stderrR, "stderr") }()
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
			delivery.mu.Lock()
			delivery.state.Updated = at.UTC()
			delivery.mu.Unlock()
			snapshots.offer(delivery.snapshot())
		}
	}
	ended := time.Now().UTC()
	exit := cmd.ProcessState.ExitCode()
	delivery.mu.Lock()
	delivery.state.SourceAlive = false
	delivery.state.SourceExit = &exit
	delivery.state.SourceEnded = &ended
	delivery.state.Updated = ended
	delivery.state.CleanupInProgress = true
	delivery.mu.Unlock()
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	var drainErr error
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for remaining := 2; remaining > 0; {
		select {
		case err := <-drains:
			drainErr = errors.Join(drainErr, err)
			remaining--
		case <-timer.C:
			stdoutR.Close()
			stderrR.Close()
			drainErr = errors.Join(drainErr, errors.New("monitor producer drain exceeded bound"))
			for remaining > 0 {
				drainErr = errors.Join(drainErr, <-drains)
				remaining--
			}
		}
	}
	sinkErr := delivery.close()
	closed = true
	finished := time.Now().UTC()
	delivery.mu.Lock()
	delivery.state.CleanupInProgress = false
	delivery.state.Finished = &finished
	delivery.state.Updated = finished
	delivery.state.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	delivery.state.Interrupted = ctx.Err() != nil && !delivery.state.TimedOut
	delivery.state.CleanupFailed = delivery.state.CleanupFailed || drainErr != nil || sinkErr != nil
	delivery.mu.Unlock()
	return errors.Join(commandErr, drainErr, sinkErr, ctx.Err())
}

// This queue replaces obsolete status snapshots; it is not a log queue. File
// writes run outside the source supervisor so failure cannot block cancellation.
type monitorSnapshots struct {
	queue    chan monitorState
	done     chan error
	delivery *monitorDelivery
}

func newMonitorSnapshots(root *os.Root, delivery *monitorDelivery) *monitorSnapshots {
	snapshots := &monitorSnapshots{queue: make(chan monitorState, 1), done: make(chan error, 1), delivery: delivery}
	go func() {
		var outcome error
		for state := range snapshots.queue {
			if outcome == nil {
				outcome = writeMonitorState(root, state)
			}
			if outcome != nil {
				delivery.mu.Lock()
				delivery.state.SnapshotFailed = true
				delivery.mu.Unlock()
			}
		}
		snapshots.done <- outcome
	}()
	return snapshots
}
func (s *monitorSnapshots) offer(state monitorState) {
	select {
	case s.queue <- state:
		return
	default:
	}
	select {
	case <-s.queue:
	default:
	}
	s.queue <- state // one supervisor owns all offers; capacity is now available
}
func (s *monitorSnapshots) close() error {
	close(s.queue)
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case err := <-s.done:
		return err
	case <-timer.C:
		s.delivery.mu.Lock()
		s.delivery.state.CleanupFailed = true
		s.delivery.mu.Unlock()
		return errors.New("monitor snapshot join exceeded bound")
	}
}

func validMonitorName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// The source and its producer drains have joined before this terminal-view stage.
// Retained files continue to expire until the explicitly selected panel stops.
func maintainMonitorLogs(ctx context.Context, root *os.Root, delivery *monitorDelivery, viewDone <-chan struct{}) (outcome error) {
	prior := delivery.snapshot()
	var store *logStore
	var err error
	if !prior.SinksJoined {
		err = errors.New("retention cannot reopen unjoined log sinks")
	} else {
		store, err = openLogStore(delivery.store.root.Name(), delivery.store.policy, time.Now().UTC())
	}
	if err != nil {
		delivery.mu.Lock()
		delivery.state.RetentionFailed = true
		delivery.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-viewDone:
		}
		return err
	}
	// Reopening is maintenance within this session: preserve delivery counters,
	// but inventory actual retained bytes and add newly expired files.
	store.stats.ReceivedBytes = prior.Logs.ReceivedBytes
	store.stats.WrittenBytes = prior.Logs.WrittenBytes
	store.stats.LostBytes = prior.Logs.LostBytes
	store.stats.ExpiredBytes += prior.Logs.ExpiredBytes
	delivery.store = store
	delivery.console = nil
	delivery.fileQueue = make(chan monitorRecord, 128)
	delivery.consoleQueue = make(chan []byte, 64)
	delivery.fileDone = make(chan error, 1)
	delivery.consoleDone = make(chan error, 1)
	delivery.mu.Lock()
	delivery.state.RetentionActive = true
	delivery.state.SinksJoined = false
	delivery.state.Logs = store.Stats()
	delivery.mu.Unlock()
	go delivery.runFile()
	go delivery.runConsole()
	snapshots := newMonitorSnapshots(root, delivery)
	defer func() {
		sinkErr := delivery.close()
		delivery.mu.Lock()
		delivery.state.RetentionActive = false
		delivery.state.Updated = time.Now().UTC()
		delivery.state.CleanupFailed = delivery.state.CleanupFailed || sinkErr != nil
		delivery.mu.Unlock()
		snapshots.offer(delivery.snapshot())
		outcome = errors.Join(outcome, sinkErr, snapshots.close())
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-viewDone:
			return nil
		case at := <-tick.C:
			delivery.mu.Lock()
			delivery.state.Updated = at.UTC()
			delivery.state.RetentionFailed = delivery.state.RetentionFailed || delivery.state.Logs.Failed
			delivery.mu.Unlock()
			snapshots.offer(delivery.snapshot())
		}
	}
}
