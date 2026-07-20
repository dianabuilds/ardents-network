package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const ReportDirEnv = "ARDENTS_TESTKIT_REPORT_DIR"

type Layer string

const (
	LayerIntegration Layer = "integration"
	LayerE2E         Layer = "e2e"
)

type Spec struct {
	Layer       Layer    `json:"layer"`
	Domain      string   `json:"domain"`
	ScenarioID  string   `json:"scenario_id"`
	Suite       string   `json:"suite"`
	Tags        []string `json:"tags,omitempty"`
	Speed       string   `json:"speed,omitempty"`
	Environment string   `json:"environment,omitempty"`
}

type StepKind string

const (
	StepKindPrecondition StepKind = "precondition"
	StepKindStep         StepKind = "step"
	StepKindAssertion    StepKind = "assertion"
	StepKindDegraded     StepKind = "degraded"
)

type StepReport struct {
	Kind      StepKind      `json:"kind"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Duration  time.Duration `json:"duration"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
}

type Report struct {
	Package   string        `json:"package"`
	Spec      Spec          `json:"spec"`
	TestName  string        `json:"test_name"`
	Status    string        `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Duration  time.Duration `json:"duration"`
	Steps     []StepReport  `json:"steps"`
}

type Scenario struct {
	t      *testing.T
	report *Report
}

func BeginScenario(t *testing.T, spec Spec) *Scenario {
	t.Helper()
	require.NoError(t, spec.Validate())

	scenario := &Scenario{
		t: t,
		report: &Report{
			Package:   callerPackage(2),
			Spec:      spec,
			TestName:  t.Name(),
			Status:    "running",
			StartedAt: time.Now().UTC(),
		},
	}

	t.Cleanup(func() {
		scenario.finalize()
	})

	return scenario
}

func (s *Scenario) Spec() Spec {
	s.t.Helper()
	return s.report.Spec
}

func (s *Scenario) Precondition(name string, fn func(t *testing.T)) {
	s.t.Helper()
	s.run(StepKindPrecondition, name, fn)
}

func (s *Scenario) Step(name string, fn func(t *testing.T)) {
	s.t.Helper()
	s.run(StepKindStep, name, fn)
}

func (s *Scenario) Assert(name string, fn func(t *testing.T)) {
	s.t.Helper()
	s.run(StepKindAssertion, name, fn)
}

func (s *Scenario) Degraded(name string, fn func(t *testing.T)) {
	s.t.Helper()
	s.run(StepKindDegraded, name, fn)
}

func (s *Scenario) run(kind StepKind, name string, fn func(t *testing.T)) {
	s.t.Helper()

	startedAt := time.Now().UTC()
	failedBefore := s.t.Failed()
	s.t.Logf("%s: %s", kind, name)

	defer func() {
		endedAt := time.Now().UTC()
		status := "passed"
		if s.t.Failed() && !failedBefore {
			status = "failed"
		}
		s.report.Steps = append(s.report.Steps, StepReport{
			Kind:      kind,
			Name:      name,
			Status:    status,
			Duration:  endedAt.Sub(startedAt),
			StartedAt: startedAt,
			EndedAt:   endedAt,
		})
	}()

	fn(s.t)
}

func (s *Scenario) finalize() {
	s.report.EndedAt = time.Now().UTC()
	s.report.Duration = s.report.EndedAt.Sub(s.report.StartedAt)
	if s.t.Failed() {
		s.report.Status = "failed"
	} else {
		s.report.Status = "passed"
	}

	payload, err := json.Marshal(s.report)
	if err != nil {
		s.t.Logf("testkit report marshal failed: %v", err)
		return
	}

	s.t.Logf("testkit-report=%s", payload)

	reportDir := strings.TrimSpace(os.Getenv(ReportDirEnv))
	if reportDir == "" {
		return
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		s.t.Logf("testkit report dir create failed: %v", err)
		return
	}

	name := sanitizeReportName(s.t.Name())
	path := filepath.Join(reportDir, fmt.Sprintf("%s-%d.json", name, time.Now().UnixNano()))
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		s.t.Logf("testkit report write failed: %v", err)
		return
	}

	s.t.Logf("testkit-report-file=%s", path)
}

func (s Spec) Validate() error {
	if s.Layer == "" {
		return fmt.Errorf("layer is required")
	}
	if s.Layer != LayerIntegration && s.Layer != LayerE2E {
		return fmt.Errorf("unsupported layer %q", s.Layer)
	}
	if strings.TrimSpace(s.Domain) == "" {
		return fmt.Errorf("domain is required")
	}
	if strings.TrimSpace(s.ScenarioID) == "" {
		return fmt.Errorf("scenario id is required")
	}
	if strings.TrimSpace(s.Suite) == "" {
		return fmt.Errorf("suite is required")
	}
	for _, tag := range s.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("tags must not contain empty values")
		}
	}
	return nil
}

func sanitizeReportName(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		" ", "-",
	)
	return replacer.Replace(name)
}

func callerPackage(skip int) string {
	_, file, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	dir := filepath.Dir(file)
	root := findModuleRoot(dir)
	if root == "" {
		return filepath.ToSlash(dir)
	}

	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}

func findModuleRoot(dir string) string {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
