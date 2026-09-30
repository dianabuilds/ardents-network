//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Reports are fixed-category projections of private evidence. Both CLI and HTTP
// call assessRun; neither adapter interprets errors or executes suggestions.
type runReport struct {
	Status          string            `json:"status"`
	Complete        bool              `json:"complete"`
	Finished        bool              `json:"finished"`
	ExitCode        int               `json:"exit_code"`
	TimedOut        bool              `json:"timed_out"`
	Interrupted     bool              `json:"interrupted"`
	SourceChanged   bool              `json:"source_changed"`
	DurationSeconds float64           `json:"duration_seconds"`
	Conditions      reportConditions  `json:"conditions"`
	FirstObserved   *reportFailure    `json:"first_observed_failure,omitempty"`
	Failures        []reportFailure   `json:"failures"`
	ResourceFacts   map[string]uint64 `json:"resource_facts"`
	Gaps            []string          `json:"gaps"`
	GapDescriptions map[string]string `json:"gap_descriptions"`
	NextChecks      []reportCheck     `json:"next_checks"`
}
type reportConditions struct {
	Source        string  `json:"source_revision,omitempty"`
	Tree          string  `json:"source_tree,omitempty"`
	Image         string  `json:"image,omitempty"`
	Go            string  `json:"go,omitempty"`
	BudgetSeconds float64 `json:"budget_seconds,omitempty"`
	// Old arbitrary commands omit declaration rather than guessing their workload.
	Mode      string            `json:"mode,omitempty"`
	Race      *bool             `json:"race,omitempty"`
	Profiling *bool             `json:"profiling,omitempty"`
	Tools     map[string]string `json:"tools,omitempty"`
}
type reportFailure struct {
	Owner      string    `json:"owner"`
	Kind       string    `json:"kind"`
	State      string    `json:"state,omitempty"`
	Category   string    `json:"category,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	// Ordinal is local collector order, never a cross-role causal order.
	Record  int        `json:"record"`
	OwnerAt *time.Time `json:"owner_at,omitempty"`
}
type reportCheck struct {
	ID            string `json:"id"`
	Reason        string `json:"reason"`
	Prerequisites string `json:"prerequisites"`
	Budget        string `json:"budget"`
	Sensitive     bool   `json:"sensitive"`
	Invocation    string `json:"invocation"`
	Expected      string `json:"expected"`
}
type assistantReport struct {
	Run        runReport         `json:"run"`
	Comparison *reportComparison `json:"comparison,omitempty"`
}
type reportComparison struct {
	Other       runReport `json:"other"`
	Conditions  string    `json:"conditions"`
	Differences []string  `json:"differences"`
	// Display observed wall durations; deliberately no acceleration verdict.
	DurationDifferenceSeconds float64 `json:"duration_difference_seconds"`
}

var hexRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
var hexTree = regexp.MustCompile(`^[0-9a-f]{64}$`)
var imageDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var goVersion = regexp.MustCompile(`^go version go[0-9]{1,2}\.[0-9]{1,2}(\.[0-9]{1,2})? linux/amd64$`)

func (r *runReport) gap(code string) {
	for _, old := range r.Gaps {
		if old == code {
			return
		}
	}
	r.Gaps = append(r.Gaps, code)
	if r.GapDescriptions == nil {
		r.GapDescriptions = map[string]string{}
	}
	r.GapDescriptions[code] = gapExplanation(code)
}
func openEvidenceRoot(dir string) (*os.Root, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, errors.New("select canonical absolute evidence directory")
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil || canonical != dir {
		return nil, errors.New("evidence directory unavailable or contains symlinks")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, errors.New("evidence directory unavailable")
	}
	return root, nil
}

// os.Root confines every named file. NONBLOCK prevents a malicious FIFO from
// blocking admission, NOFOLLOW refuses a replacement symlink, Stat admits only
// regular files, and byte bounds apply even if a file grows after Stat.
func readEvidence(root *os.Root, name string, limit int64) ([]byte, time.Time, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, time.Time{}, errors.Join(errors.New("evidence requires regular file"), err, f.Close())
	}
	if info.Size() > limit {
		return nil, time.Time{}, errors.Join(errors.New("evidence byte limit exceeded"), f.Close())
	}
	body, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	closeErr := f.Close()
	if int64(len(body)) > limit {
		return nil, time.Time{}, errors.New("evidence byte limit exceeded")
	}
	return body, info.ModTime(), errors.Join(readErr, closeErr)
}

// Reject duplicate JSON keys (including nested objects) rather than allowing
// last-write-wins records to silently contradict earlier evidence.
func validJSONRecord(body []byte) bool {
	d := json.NewDecoder(bytes.NewReader(body))
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate or invalid key")
				}
				seen[name] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid delimiter")
		}
		_, err = d.Token()
		return err
	}
	if len(body) > lineLimit && len(body) > recordLimit {
		return false
	}
	// Bound nesting before the recursive walk.
	depth := 0
	tokens := json.NewDecoder(bytes.NewReader(body))
	for {
		t, err := tokens.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false
		}
		if v, ok := t.(json.Delim); ok {
			if v == '{' || v == '[' {
				depth++
				if depth > 32 {
					return false
				}
			} else {
				depth--
			}
		}
	}
	if err := value(); err != nil {
		return false
	}
	_, err := d.Token()
	return errors.Is(err, io.EOF)
}

func assessRun(dir string, now time.Time) (result runReport) {
	result.Status = "unavailable"
	result.ExitCode = -1
	result.ResourceFacts = map[string]uint64{}
	result.Failures = []reportFailure{}
	result.Gaps = []string{}
	result.NextChecks = []reportCheck{}
	defer func() {
		result.NextChecks = nextChecks(result)
	}()
	root, err := openEvidenceRoot(dir)
	if err != nil {
		result.gap("evidence-unavailable")
		return
	}
	defer func() {
		if root.Close() != nil {
			result.gap("evidence-close-failed")
			result.Complete = false
		}
	}()
	var manifest struct {
		Schema    string `json:"schema"`
		Source    string `json:"source_sha"`
		Tree      string `json:"source_tree_sha256"`
		Image     string `json:"image"`
		Go        string `json:"go"`
		Timeout   string `json:"timeout"`
		Mode      string `json:"mode"`
		Race      *bool  `json:"race"`
		Profiling *bool  `json:"profiling"`
	}
	body, _, err := readEvidence(root, "manifest.json", lineLimit)
	manifestOK := err == nil && validJSONRecord(body) && json.Unmarshal(body, &manifest) == nil && manifest.Schema == "ardents-local-diagnostics-v1"
	if !manifestOK {
		result.gap("manifest-unavailable-or-invalid")
	} else {
		c := &result.Conditions
		if hexRevision.MatchString(manifest.Source) {
			c.Source = manifest.Source
		} else {
			result.gap("source-revision-unavailable")
		}
		if hexTree.MatchString(manifest.Tree) {
			c.Tree = manifest.Tree
		} else {
			result.gap("source-content-unavailable")
		}
		if imageDigest.MatchString(manifest.Image) {
			c.Image = manifest.Image
		} else {
			result.gap("image-unavailable")
		}
		if goVersion.MatchString(manifest.Go) {
			c.Go = manifest.Go
		} else {
			result.gap("compiler-unavailable")
		}
		if d, err := time.ParseDuration(manifest.Timeout); err == nil && d > 0 && d <= 24*time.Hour {
			c.BudgetSeconds = d.Seconds()
		} else {
			result.gap("run-budget-unavailable")
		}
		c.Mode = selected(manifest.Mode, "run test static")
		if c.Mode == "test" {
			c.Race = manifest.Race
			c.Profiling = manifest.Profiling
		}
	}
	// No existing manifest declares full workload/environment identity.
	result.gap("workload-and-environment-not-fully-declared")
	readReportTools(root, &result)
	name := "summary.json"
	body, modified, err := readEvidence(root, name, recordLimit)
	if errors.Is(err, os.ErrNotExist) {
		name = "live.json"
		body, modified, err = readEvidence(root, name, recordLimit)
	}
	var s summary
	var keys map[string]json.RawMessage
	if err != nil || !validJSONRecord(body) || json.Unmarshal(body, &s) != nil || json.Unmarshal(body, &keys) != nil {
		result.gap("summary-unavailable-or-invalid")
		return
	}
	for _, field := range strings.Fields("started exit_code timed_out interrupted source_tree_changed capture_incomplete samples counts peak") {
		if raw, ok := keys[field]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			result.gap("summary-required-fields-missing")
			return
		}
	}
	if s.Started.IsZero() || s.Started.After(now.Add(time.Minute)) || s.ExitCode < -1 || s.ExitCode > 255 || s.TimedOut && s.Interrupted || s.StdoutBytes < 0 || s.StderrBytes < 0 || s.RawDroppedBytes < 0 || s.EventDroppedBytes < 0 || s.SampleDroppedBytes < 0 {
		result.gap("summary-invalid-values")
		return
	}
	result.Finished = !s.Finished.IsZero()
	if name == "summary.json" && !result.Finished || result.Finished && (s.Finished.Before(s.Started) || s.Finished.Sub(s.Started) > 24*time.Hour+time.Minute || s.Finished.After(now.Add(time.Minute))) {
		result.gap("summary-invalid-lifetime")
		return
	}
	result.Complete = manifestOK && !s.CaptureIncomplete && !s.SourceChanged && s.SampleFailures == 0 && s.OversizedLines == 0 && s.UnknownCategories == 0 && s.RawDroppedBytes == 0 && s.EventDroppedBytes == 0 && s.SampleDroppedBytes == 0
	result.ExitCode = s.ExitCode
	result.TimedOut = s.TimedOut
	result.Interrupted = s.Interrupted
	result.SourceChanged = s.SourceChanged
	if result.Finished {
		result.DurationSeconds = s.Finished.Sub(s.Started).Seconds()
		result.Status = "command-completed"
		if s.ExitCode != 0 {
			result.Status = "command-failed"
		}
	} else {
		result.Status = "collecting"
		if now.Sub(modified) > 10*time.Second || modified.After(now.Add(time.Minute)) {
			result.Status = "observation-stale"
			result.Complete = false
			result.gap("live-observation-stale")
		}
	}
	if !result.Complete {
		result.gap("capture-incomplete")
	}
	if s.SourceChanged {
		result.gap("source-changed")
	}
	if s.TimedOut {
		result.Status = "timed-out"
	} else if s.Interrupted {
		result.Status = "interrupted"
	}
	result.gap("current-capability-readiness-not-established")
	result.gap("stage-durations-queues-credit-token-stock-unavailable")
	readReportEvents(root, &result, s.Counts)
	readReportSamples(root, &result, s.Samples)
	if result.FirstObserved == nil {
		result.gap("first-owner-failure-not-observed")
	}
	result.gap("observed-order-is-not-causality")
	if !result.Complete && result.Status == "command-completed" {
		result.Status = "command-completed-with-incomplete-evidence"
	}
	return
}

func readReportEvents(root *os.Root, r *runReport, expected map[string]uint64) {
	body, _, err := readEvidence(root, "events.ndjson", recordLimit)
	if err != nil {
		r.gap("event-history-unavailable")
		r.Complete = false
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), lineLimit)
	record := 0
	seen := map[string]uint64{}
	var previous time.Time
	for scanner.Scan() {
		record++
		if record > 65536 {
			r.gap("event-history-invalid-or-truncated")
			r.Complete = false
			return
		}
		var e event
		row := scanner.Bytes()
		if !validJSONRecord(row) || json.Unmarshal(row, &e) != nil || !safeReportEvent(e) {
			r.gap("event-history-invalid-or-truncated")
			r.Complete = false
			continue
		}
		seen[e.Kind]++
		if e.ObservedAt.After(time.Now().Add(time.Minute)) {
			r.gap("collector-time-order-uncertain")
			r.Complete = false
		}
		if e.ObservedAt.Before(previous) {
			r.gap("collector-time-order-uncertain")
		}
		previous = e.ObservedAt
		if e.UnknownCategories > 0 {
			r.gap("owner-category-unclassified")
			r.Complete = false
		}
		if e.Failure != "" || e.State == "FAILED" || strings.HasSuffix(e.Kind, "-failed") || e.Kind == "headless-runtime-permission-required" {
			owner := "Node"
			if e.Schema == "ardents-source-event-v1" {
				owner = "Source"
			}
			if e.Schema == "ardents-headless-runtime-event-v1" {
				owner = "Endpoint"
			}
			f := reportFailure{Owner: owner, Kind: e.Kind, State: e.State, Category: e.Failure, ObservedAt: e.ObservedAt, Record: record}
			if at, err := time.Parse(time.RFC3339Nano, e.At); err == nil {
				f.OwnerAt = &at
			}
			if r.FirstObserved == nil {
				r.FirstObserved = &f
			}
			if len(r.Failures) < 8 {
				r.Failures = append(r.Failures, f)
			} else {
				r.gap("failure-list-truncated")
			}
		}
	}
	if scanner.Err() != nil || len(body) > 0 && body[len(body)-1] != '\n' {
		r.gap("event-history-invalid-or-truncated")
		r.Complete = false
	}
	if r.Finished {
		for key, value := range expected {
			if seen[key] != value {
				r.gap("event-history-count-mismatch")
				r.Complete = false
			}
		}
		for key, value := range seen {
			if expected[key] != value {
				r.gap("event-history-count-mismatch")
				r.Complete = false
			}
		}
	}

}

func safeReportEvent(e event) bool {
	if e.ObservedAt.IsZero() {
		return false
	}
	kinds := ""
	switch e.Schema {
	case "ardents-node-event-v1":
		kinds = "lifecycle resource resource-sample"
	case "ardents-source-event-v1":
		kinds = "source-ready source-wave-accepted source-failed"
	case "ardents-headless-runtime-event-v1":
		kinds = "headless-runtime-ready headless-runtime-failed headless-runtime-permission-required headless-runtime-publication-refresh-failed headless-runtime-publication-withdrawal-failed headless-runtime-connection-operation-failed"
	default:
		return false
	}
	if selected(e.Kind, kinds) == "" {
		return false
	}
	if e.State != "" && selected(e.State, "ABSENT PREPARED READY DRAINING WITHDRAWN FAILED OBSERVED NORMAL PROTECT DRAIN EXIT") == "" {
		return false
	}
	if e.Carrier != "" && selected(e.Carrier, "ardents-carrier-tcp-tls-v2 ardents-carrier-quic-v2") == "" {
		return false
	}
	failures := "unclassified"
	if e.Schema == "ardents-source-event-v1" {
		failures += " background-work cleanup"
	}
	if e.Schema == "ardents-headless-runtime-event-v1" {
		failures += " startup running rotation authorization publication-state publication-handover caller-context publisher-ended publication-draining registration-absent publisher-not-live registration publisher-drain deadline admission activation worker-launch worker-operation introduction-preparation service-join post-join-lifetime service-result rotation-authority rotation-prefix rotation-recipient rotation-expired rotation-registration rotation-publication"
	}
	return e.Failure == "" || selected(e.Failure, failures) != ""
}

func readReportSamples(root *os.Root, r *runReport, expected uint64) {
	body, _, err := readEvidence(root, "samples.ndjson", recordLimit)
	if err != nil {
		r.gap("resource-samples-unavailable")
		r.Complete = false
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), lineLimit)
	rows := 0
	for scanner.Scan() {
		rows++
		var s processSample
		if rows > 65536 {
			r.gap("resource-samples-invalid-or-truncated")
			r.Complete = false
			return
		}
		if !validJSONRecord(scanner.Bytes()) || json.Unmarshal(scanner.Bytes(), &s) != nil || s.At.IsZero() {
			r.gap("resource-samples-invalid-or-truncated")
			r.Complete = false
			continue
		}
		for _, key := range strings.Fields("memory.current memory.max pids.current pids.max memory.events:max memory.events:high memory.events:oom memory.events:oom_kill cpu.stat:throttled_usec cpu.stat:nr_throttled") {
			if value, ok := s.Cgroup[key]; ok {
				if value > r.ResourceFacts[key] {
					r.ResourceFacts[key] = value
				}
			}
		}
		if s.RSSBytes > r.ResourceFacts["process-group-rss-peak"] {
			r.ResourceFacts["process-group-rss-peak"] = s.RSSBytes
		}
		if len(s.Unavailable) > 0 {
			r.gap("some-resource-observations-unavailable")
		}
		for _, n := range s.Pressure {
			if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
				r.gap("resource-samples-invalid-or-truncated")
				r.Complete = false
			}
		}
	}
	if scanner.Err() != nil || len(body) > 0 && body[len(body)-1] != '\n' {
		r.gap("resource-samples-invalid-or-truncated")
		r.Complete = false
	}
	if r.Finished && uint64(rows) != expected {
		r.gap("resource-sample-count-mismatch")
		r.Complete = false
	}
	if rows == 0 {
		r.gap("resource-samples-not-observed")
	}
	r.gap("container-counters-not-attributed-to-owner")
}

func nextChecks(r runReport) []reportCheck {
	checks := []reportCheck{}
	add := func(id, reason, prerequisites, budget, expected string, sensitive bool) {
		checks = append(checks, reportCheck{ID: id, Reason: reason, Prerequisites: prerequisites, Budget: budget, Sensitive: sensitive, Expected: expected, Invocation: checkInvocation(id)})
	}
	if r.Status == "unavailable" || !r.Complete {
		add("doctor-and-complete-capture", "Данные отсутствуют или неполны; заключение требует нового полного прогона.", "Явно собранный диагностический образ; новая приватная директория.", "doctor: 10 секунд на инструмент; повторный прогон: явный лимит до 24 часов.", "Сохранённые manifest, terminal summary и счётчики потерь; исходный FAIL остаётся сохранённым.", false)
	}
	if r.ResourceFacts["memory.events:max"] > 0 || r.ResourceFacts["memory.events:oom"] > 0 || r.ResourceFacts["memory.events:oom_kill"] > 0 || r.ResourceFacts["cpu.stat:nr_throttled"] > 0 {
		add("resource-control", "Наблюдались ненулевые общие cgroup counters; причинная связь с отказом не установлена.", "Та же объявленная нагрузка; отдельный контроль с зафиксированными бюджетами.", "Конечный лимит времени и памяти; 1-секундные samples.", "Изменение счётчиков относительно начала и результат команды; принадлежность ресурса владельцу проверяется отдельно.", false)
	}
	if r.FirstObserved != nil {
		add("bounded-owner-profile", "Наблюдался отказ; stacks/CPU profile могут уточнить гипотезу, но не доказывают её.", "Оператор выбирает свой живой debug socket; требуется explicit sensitive capture.", "Один профиль, CPU/trace 1–30 секунд, 64 MiB; отдельный контроль без profiling.", "Сохранённый приватный профиль и воспроизводимая owner-level проверка гипотезы.", true)
		add("owner-race-control", "Есть явно наблюдаемый отказ владельца; следующее доказательство ищется на его границе.", "Оператор выбирает реальный affected-owner test из текущего кода; отдельный race run.", "Один пакет, явный test pattern, timeout до 24 часов; исходные отказы сохраняются.", "Воспроизводимая положительная/отрицательная проверка причины; PASS повтора не стирает FAIL.", false)
	}
	if r.Status == "collecting" {
		add("bounded-runtime-snapshot", "Прогон ещё выполняется; runtime observations могут уточнить локальное состояние процесса.", "Явно включённый принадлежащий оператору private Unix debug socket.", "Runtime request: 40 секунд клиента, не более 64 MiB.", "Go-runtime counters; текущая product readiness остаётся отдельным фактом.", false)
	}
	if r.Status == "observation-stale" {
		add("verify-owned-collector", "Live snapshot старше 10 секунд; отсутствие свежих данных не доказывает остановку продукта.", "Проверить известный handle только собственного диагностического процесса.", "Конечная локальная проверка; не запускать второй collector поверх прежнего.", "Установлен статус collector и отдельный статус supervised command.", false)
	}
	if r.Finished {
		add("test-timings", "Время supervisor включает подготовку и сбор; оно не равно времени выполнения теста.", "Сохранённый полный приватный go test -json output; отсутствие skips отдельно проверено.", "Вход timings: 64 MiB, 500000 строк; 20 верхних результатов.", "Отдельные package/top-level test durations без суммирования parent/subtest.", true)
	}
	if r.Conditions.Profiling != nil && *r.Conditions.Profiling {
		add("unprofiled-control", "Профилирование меняет scheduling и затраты; performance вывод требует отдельного контроля.", "Та же объявленная нагрузка и окружение, profiling выключен.", "Отдельный конечный прогон в новом каталоге.", "Контрольные результаты без overhead profiling.", false)
	}
	return checks
}

func buildAssistantReport(dir, compare string, now time.Time) assistantReport {
	report := assistantReport{Run: assessRun(dir, now)}
	if compare == "" {
		return report
	}
	other := assessRun(compare, now)
	c := &reportComparison{Other: other, Conditions: "unknown", Differences: []string{}}
	a, b := report.Run.Conditions, other.Conditions
	for _, pair := range []struct{ name, a, b string }{
		{"source-content", a.Tree, b.Tree}, {"image", a.Image, b.Image}, {"compiler", a.Go, b.Go}, {"mode", a.Mode, b.Mode},
	} {
		if pair.a != "" && pair.b != "" && pair.a != pair.b {
			c.Differences = append(c.Differences, pair.name)
		}
	}
	if a.BudgetSeconds > 0 && b.BudgetSeconds > 0 && a.BudgetSeconds != b.BudgetSeconds {
		c.Differences = append(c.Differences, "time-budget")
	}
	for _, pair := range []struct {
		name string
		a, b *bool
	}{{"race", a.Race, b.Race}, {"profiling", a.Profiling, b.Profiling}} {
		if pair.a != nil && pair.b != nil && *pair.a != *pair.b {
			c.Differences = append(c.Differences, pair.name)
		}
	}
	for _, key := range strings.Fields("staticcheck govulncheck delve errcheck powershell") {
		if a.Tools[key] != "" && b.Tools[key] != "" && a.Tools[key] != b.Tools[key] {
			c.Differences = append(c.Differences, "tool-"+key)
		}
	}
	if len(c.Differences) > 0 {
		c.Conditions = "different"
	}
	if report.Run.Finished && other.Finished {
		c.DurationDifferenceSeconds = report.Run.DurationSeconds - other.DurationSeconds
	}
	report.Comparison = c
	return report
}
func reportCommand(args []string) error {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	dir := flags.String("dir", "", "absolute private evidence directory")
	compare := flags.String("compare", "", "optional second absolute evidence directory")
	asJSON := flags.Bool("json", false, "safe structured explanation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 || !filepath.IsAbs(*dir) || *compare != "" && !filepath.IsAbs(*compare) {
		return errors.New("select absolute -dir and optional -compare")
	}
	report := buildAssistantReport(*dir, *compare, time.Now().UTC())
	if *asJSON {
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return err
		}
	} else if err := writeAssistantText(os.Stdout, report); err != nil {
		return err
	}
	if report.Run.Status == "unavailable" || !report.Run.Complete || report.Comparison != nil && (!report.Comparison.Other.Complete || report.Comparison.Other.Status == "unavailable") {
		return errors.New("diagnostic explanation has incomplete evidence")
	}
	return nil // A successfully explained command FAIL remains FAIL in the report.
}
func writeAssistantText(w io.Writer, r assistantReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Ardents · помощник диагностики\nИтог: %s; exit=%d; полнота=%t; timeout=%t; interruption=%t\n", r.Run.Status, r.Run.ExitCode, r.Run.Complete, r.Run.TimedOut, r.Run.Interrupted)
	if r.Run.FirstObserved != nil {
		f := r.Run.FirstObserved
		fmt.Fprintf(&b, "Первый доступный отказ: %s / %s / %s, запись events.ndjson #%d. Это наблюдение, не доказанная первопричина.\n", f.Owner, f.Kind, f.Category, f.Record)
	}
	conditions, _ := json.Marshal(r.Run.Conditions)
	fmt.Fprintf(&b, "Условия: %s\n", conditions)
	resources, _ := json.Marshal(r.Run.ResourceFacts)
	fmt.Fprintf(&b, "Ресурсные факты: %s\n", resources)
	for _, f := range r.Run.Failures {
		fmt.Fprintf(&b, "Наблюдение events.ndjson #%d: %s / %s / %s / %s\n", f.Record, f.Owner, f.Kind, f.State, f.Category)
	}
	fmt.Fprintf(&b, "Время supervisor: %.3f с (включает подготовку и сбор).\n", r.Run.DurationSeconds)
	for _, gap := range r.Run.Gaps {
		fmt.Fprintf(&b, "Пробел: %s [%s]\n", gapExplanation(gap), gap)
	}
	if r.Comparison != nil {
		fmt.Fprintf(&b, "Сравнение: условия=%s; различия=%s; разность supervisor duration=%.3f с; прежний итог=%s (exit=%d). Вывод об ускорении не установлен.\n", r.Comparison.Conditions, strings.Join(r.Comparison.Differences, ","), r.Comparison.DurationDifferenceSeconds, r.Comparison.Other.Status, r.Comparison.Other.ExitCode)
	}
	for _, check := range r.Run.NextChecks {
		fmt.Fprintf(&b, "\nСледующая проверка [%s]: %s\nУсловия: %s\nЛимит: %s\nЧувствительные данные: %t\nОжидаемый результат: %s\n", check.ID, check.Reason, check.Prerequisites, check.Budget, check.Sensitive, check.Expected)
		fmt.Fprintf(&b, "Шаблон команды (параметры выбирает оператор): %s\n", check.Invocation)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func safeSummaryForPanel(s summary) (summary, error) {
	if len(s.LastEvents) > 256 {
		return summary{}, errors.New("event tail exceeds bound")
	}
	clean := s
	clean.Counts = map[string]uint64{}
	for _, k := range strings.Fields("lifecycle resource resource-sample source-ready source-wave-accepted source-failed headless-runtime-failed headless-runtime-ready headless-runtime-permission-required headless-runtime-publication-refresh-failed headless-runtime-publication-withdrawal-failed headless-runtime-connection-operation-failed") {
		if n, ok := s.Counts[k]; ok {
			clean.Counts[k] = n
		}
	}
	clean.Peak = map[string]float64{}
	for _, k := range resourceFields {
		if n, ok := s.Peak[k]; ok && n >= 0 && !math.IsInf(n, 0) && !math.IsNaN(n) {
			clean.Peak[k] = n
		}
	}
	clean.LastEvents = nil
	cleanEvent := func(e event) (event, error) {
		if !safeReportEvent(e) {
			return event{}, errors.New("event category unavailable")
		}
		e.Resource = filterReportNumbers(e.Resource, resourceFields)
		e.Hosting = filterReportNumbers(e.Hosting, hostingFields)
		if t, err := time.Parse(time.RFC3339Nano, e.At); err == nil {
			e.At = t.UTC().Format(time.RFC3339Nano)
		} else {
			e.At = ""
		}
		return e, nil
	}
	for _, e := range s.LastEvents {
		c, err := cleanEvent(e)
		if err != nil {
			return summary{}, err
		}
		clean.LastEvents = append(clean.LastEvents, c)
	}
	if s.LastResource != nil {
		e, err := cleanEvent(*s.LastResource)
		if err != nil {
			return summary{}, err
		}
		clean.LastResource = &e
	}
	return clean, nil
}
func filterReportNumbers(input map[string]float64, keys []string) map[string]float64 {
	output := map[string]float64{}
	for _, k := range keys {
		if v, ok := input[k]; ok && v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			output[k] = v
		}
	}
	return output
}
func safeSamplesForPanel(dir string) ([]processSample, error) {
	root, err := openEvidenceRoot(dir)
	if err != nil {
		return nil, err
	}
	body, _, err := readEvidence(root, "samples.ndjson", recordLimit)
	err = errors.Join(err, root.Close())
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), lineLimit)
	rows := []processSample{}
	for scanner.Scan() {
		if len(rows) >= 65536 || !validJSONRecord(scanner.Bytes()) {
			return nil, errors.New("sample record invalid")
		}
		var s processSample
		if json.Unmarshal(scanner.Bytes(), &s) != nil || s.At.IsZero() {
			return nil, errors.New("sample record invalid")
		}
		cg := map[string]uint64{}
		for _, k := range strings.Fields("memory.current memory.max pids.current pids.max memory.events:max memory.events:high memory.events:oom memory.events:oom_kill cpu.stat:usage_usec cpu.stat:throttled_usec cpu.stat:nr_throttled") {
			if v, ok := s.Cgroup[k]; ok {
				cg[k] = v
			}
		}
		network := map[string]uint64{}
		for _, k := range strings.Fields("Tcp:RetransSegs Tcp:InErrs Tcp:OutRsts Tcp:CurrEstab Udp:InErrors Udp:RcvbufErrors Udp:SndbufErrors") {
			if v, ok := s.Network[k]; ok {
				network[k] = v
			}
		}
		s.Cgroup = cg
		s.Network = network
		s.Pressure = filterReportNumbers(s.Pressure, strings.Fields("cpu memory io"))
		unavailable := []string{}
		for _, v := range s.Unavailable {
			if selected(v, "some_process_fds memory.current memory.max pids.current pids.max cpu.stat memory.events cpu.pressure memory.pressure io.pressure") != "" {
				unavailable = append(unavailable, v)
			}
		}
		s.Unavailable = unavailable
		rows = append(rows, s)
	}
	if scanner.Err() != nil || len(body) > 0 && body[len(body)-1] != '\n' {
		return nil, errors.New("sample history truncated")
	}
	return rows, nil
}

func readReportTools(root *os.Root, r *runReport) {
	r.Conditions.Tools = map[string]string{}
	body, _, err := readEvidence(root, "tools.txt", 1<<20)
	if err != nil {
		r.gap("tool-versions-unavailable")
		return
	}
	patterns := []struct {
		key     string
		pattern *regexp.Regexp
	}{
		{"staticcheck", regexp.MustCompile(`^staticcheck ([0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2}) \([0-9.]+\)$`)},
		{"govulncheck", regexp.MustCompile(`^Scanner: govulncheck@(v[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2})$`)},
		{"delve", regexp.MustCompile(`^\s*mod\s+github.com/go-delve/delve\s+(v[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2})(?:\s+h1:[A-Za-z0-9+/=]+)?$`)},
		{"errcheck", regexp.MustCompile(`^\s*mod\s+github.com/kisielk/errcheck\s+(v[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2})(?:\s+h1:[A-Za-z0-9+/=]+)?$`)},
		{"powershell", regexp.MustCompile(`^PowerShell ([0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2})$`)},
	}
	ambiguous := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), lineLimit)
	for scanner.Scan() {
		for _, p := range patterns {
			if parts := p.pattern.FindStringSubmatch(scanner.Text()); parts != nil {
				old := r.Conditions.Tools[p.key]
				if old != "" && old != parts[1] {
					ambiguous[p.key] = true
					r.gap("tool-versions-ambiguous")
				}
				if !ambiguous[p.key] {
					r.Conditions.Tools[p.key] = parts[1]
				} else {
					delete(r.Conditions.Tools, p.key)
				}
			}
		}
	}
	if scanner.Err() != nil {
		r.gap("tool-inventory-truncated")
	}
	if len(r.Conditions.Tools) < len(patterns) {
		r.gap("some-tool-versions-unavailable")
	}
}
func checkInvocation(id string) string {
	switch id {
	case "doctor-and-complete-capture":
		return "ardents-diagnostics doctor"
	case "resource-control":
		return "ardents-diagnostics run -out <NEW_PRIVATE_DIR> -timeout <FINITE_BUDGET> -- <SAME_DECLARED_COMMAND>"
	case "owner-race-control":
		return "ardents-diagnostics test -package <AFFECTED_PACKAGE> -run <OWNER_TEST_PATTERN> -race -out <NEW_PRIVATE_DIR> -timeout <FINITE_BUDGET>"
	case "bounded-runtime-snapshot":
		return "ardents-diagnostics snapshot -socket <OWNED_SOCKET> -kind runtime -out <NEW_PRIVATE_FILE>"
	case "bounded-owner-profile":
		return "ardents-diagnostics snapshot -socket <OWNED_LIVE_SOCKET> -kind goroutine -sensitive -out <NEW_PRIVATE_PROFILE>"
	case "test-timings":
		return "ardents-diagnostics timings -input <PRIVATE_COMPLETE_GO_JSON>"
	case "unprofiled-control":
		return "ardents-diagnostics test -package <SAME_PACKAGE> -run <SAME_PATTERN> -out <NEW_PRIVATE_DIR> -timeout <SAME_BUDGET>"
	case "verify-owned-collector":
		return "Inspect the previously recorded owned process/container handle."
	}
	return ""
}
func gapExplanation(code string) string {
	switch code {
	case "evidence-unavailable":
		return "Каталог наблюдений недоступен или не прошёл проверку."
	case "evidence-close-failed":
		return "Закрытие каталога наблюдений завершилось ошибкой."
	case "manifest-unavailable-or-invalid":
		return "Manifest отсутствует, повреждён или имеет неизвестную схему."
	case "source-revision-unavailable":
		return "Ревизия исходников не подтверждена."
	case "source-content-unavailable":
		return "Точный состав исходников не подтверждён."
	case "image-unavailable":
		return "Digest фактического Docker-образа не сохранён."
	case "compiler-unavailable":
		return "Поддерживаемый compiler/platform inventory отсутствует."
	case "run-budget-unavailable":
		return "Лимит времени прогона не установлен из manifest."
	case "workload-and-environment-not-fully-declared":
		return "Полная нагрузка и окружение не описаны; сопоставимость производительности не установлена."
	case "tool-versions-unavailable":
		return "Файл inventory инструментов недоступен."
	case "tool-versions-ambiguous":
		return "Inventory содержит противоречивые версии инструмента."
	case "tool-inventory-truncated":
		return "Inventory инструментов превышает предел строки."
	case "some-tool-versions-unavailable":
		return "Не все версии диагностических инструментов удалось безопасно распознать."
	case "summary-unavailable-or-invalid":
		return "Summary отсутствует, повреждён или противоречив."
	case "summary-required-fields-missing":
		return "В summary отсутствуют обязательные значения."
	case "summary-invalid-values":
		return "В summary недопустимые или противоречивые значения."
	case "summary-invalid-lifetime":
		return "Начало, завершение или длительность summary недопустимы."
	case "live-observation-stale":
		return "Live snapshot старше 10 секунд; статус продукта из этого не следует."
	case "capture-incomplete":
		return "Сбор наблюдений неполон; это сохраняется отдельно от результата команды."
	case "source-changed":
		return "Исходники изменились во время прогона или итоговая проверка состава не удалась."
	case "current-capability-readiness-not-established":
		return "Текущая готовность функций не доказана; исторический READY и живой процесс недостаточны."
	case "stage-durations-queues-credit-token-stock-unavailable":
		return "Нет наблюдений длительности этапов, очередей, credit и token stock."
	case "event-history-count-mismatch":
		return "Количество сохранённых событий противоречит terminal summary."
	case "resource-sample-count-mismatch":
		return "Количество сохранённых samples противоречит terminal summary."
	case "event-history-unavailable":
		return "Сохранённый журнал событий недоступен; хвост не подменяет полную историю."
	case "event-history-invalid-or-truncated":
		return "История событий содержит недопустимые или неполные записи."
	case "collector-time-order-uncertain":
		return "Времена collector идут назад; порядок наблюдений не является причинным."
	case "owner-category-unclassified":
		return "Категория отказа владельца не распознана."
	case "failure-list-truncated":
		return "Показаны первые восемь доступных отказов; остальные остаются в приватном журнале."
	case "resource-samples-unavailable":
		return "Сохранённые resource samples недоступны."
	case "resource-samples-invalid-or-truncated":
		return "Resource samples содержат недопустимые или неполные записи."
	case "some-resource-observations-unavailable":
		return "Часть resource observations недоступна."
	case "resource-samples-not-observed":
		return "Resource samples не получены; короткий прогон мог завершиться между опросами."
	case "container-counters-not-attributed-to-owner":
		return "Cgroup counters относятся к контейнеру; они не определяют виновный процесс."
	case "first-owner-failure-not-observed":
		return "Явный первый отказ владельца в доступной истории не найден."
	case "observed-order-is-not-causality":
		return "Порядок сообщений collector не доказывает причинность между ролями."
	}
	return "Наблюдение недоступно."
}
