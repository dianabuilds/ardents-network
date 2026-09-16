//go:build ignore

// Command run-issue60-checks executes only the bounded verification groups
// owned by issue #60. It lets every independent job finish and writes one
// machine-readable report instead of stopping at the first package failure.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	issue60JobTimeout    = 110 * time.Second
	issue60TestTimeout   = "90s"
	issue60MaximumOutput = 32 << 20
)

type issue60Job struct {
	Name, Package, Pattern string
	Race                   bool
}

type issue60Group struct {
	Name string
	Jobs []issue60Job
}

type issue60Event struct {
	At      time.Time       `json:"at"`
	Group   string          `json:"group"`
	Job     string          `json:"job"`
	Package string          `json:"package"`
	Kind    string          `json:"kind"`
	Record  json.RawMessage `json:"record,omitempty"`
	Failure string          `json:"failure,omitempty"`
	Elapsed time.Duration   `json:"elapsed,omitempty"`
	Passed  bool            `json:"passed,omitempty"`
}

func issue60Groups() []issue60Group {
	return []issue60Group{
		{Name: "worker-bridge", Jobs: []issue60Job{
			{Name: "fixed-worker-protocol", Package: "./internal/application/streamqualification", Pattern: "^Test", Race: true},
			{Name: "worker-command", Package: "./cmd/ardents-stream-qualification", Pattern: "^Test", Race: true},
			{Name: "endpoint-stream-ownership", Package: "./internal/endpoint", Pattern: "^(TestStreamBoundCoversSustainedWorkload|TestQualificationOwner|TestTextServiceQualifiedWorkers|TestTextServicePublisherRefusesAnotherJobsStream|TestPartialApplicationChunk|TestFinalApplicationBytes|TestCallerCancellationWins|TestSlowConsumers|TestLogicalQueue|TestOrderlyHalfClose|TestMalformedAndOversizedPublications)", Race: true},
		}},
		{Name: "installation-authority", Jobs: []issue60Job{
			{Name: "runner-evidence", Package: "./cmd/ardents-qualification", Pattern: "^(TestQualification|TestResourceVerdict)", Race: true},
			{Name: "worker-authority", Package: "./internal/endpoint", Pattern: "^(TestCompletedTextWorker|TestTextWorkerOperation|TestAlreadyCancelledTextLaunch|TestEndpointMainExit|TestTextLaunch|TestTextManager|TestTextWorkerUnit|TestTextWorkerExecutable|TestTextWorkerAttachment|TestTextWorkerControl|TestTextWorkerLifetime|TestTextWorkerInitialization|TestQualificationUbuntu)", Race: true},
		}},
		{Name: "resources", Jobs: []issue60Job{
			{Name: "hosting-ledger", Package: "./internal/resource", Pattern: "^(TestHosting|TestOwnerResident)", Race: true},
			{Name: "route-bounds", Package: "./internal/route", Pattern: "^(TestClosedForwarding|TestForwarding|TestClosedDuty|TestClosedJoin|TestClosedJoined|TestClosedSource|TestClosedOuter|TestClosedBootstrapForwarding)", Race: true},
			{Name: "node-bounds", Package: "./internal/node", Pattern: "^(TestClosedForwarding|TestDeclaredCapacity|TestEmergencyPressure|TestRendezvousPressure|TestClosedSourcePrefix)", Race: true},
		}},
		{Name: "network", Jobs: []issue60Job{
			{Name: "joined-carriers", Package: "./internal/node", Pattern: "^(TestClosedJoinNodePairsThroughBothCarriers|TestClosedResolutionPublishesAndLooksUpThroughAdmittedNodeCarrier|TestClosedIntroductionRegistrationOwnsSlotUntilExpiry)", Race: true},
			{Name: "endpoint-real-service", Package: "./internal/endpoint", Pattern: "^(TestTextWorkersReadTargetThroughJoinedNetwork|TestTextJoinedServiceTransfersDocumentThroughNetwork|TestTextRouteJoinConnectsSourceAndResponder|TestTextServiceRealTLSAndDocumentExchange)", Race: true},
		}},
	}
}

type issue60Reporter struct {
	mu      sync.Mutex
	encoder *json.Encoder
	err     error
}

func (reporter *issue60Reporter) emit(event issue60Event) error {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	if reporter.err != nil {
		return reporter.err
	}
	reporter.err = reporter.encoder.Encode(event)
	return reporter.err
}

func main() {
	if err := runIssue60Checks(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runIssue60Checks(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("run-issue60-checks", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var groupName, reportPath string
	var list bool
	flags.StringVar(&groupName, "group", "", "one group name; omit to run all groups")
	flags.StringVar(&reportPath, "report", "", "new external JSONL report path")
	flags.BoolVar(&list, "list", false, "list groups and jobs without running them")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("usage: go run ./scripts/run-issue60-checks.go [-list] [-group NAME] [-report NEW.jsonl]")
	}
	groups := issue60Groups()
	if list {
		return json.NewEncoder(output).Encode(groups)
	}
	if reportPath == "" {
		return errors.New("issue #60 checks require an explicit external report path")
	}
	selected := groups
	if groupName != "" {
		selected = nil
		for _, group := range groups {
			if group.Name == groupName {
				selected = append(selected, group)
			}
		}
		if len(selected) == 0 {
			return errors.New("unknown issue #60 check group")
		}
	}
	file, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	reporter := &issue60Reporter{encoder: json.NewEncoder(file)}
	var outcome error
	for _, group := range selected {
		outcome = errors.Join(outcome, runIssue60Group(group, reporter))
	}
	return errors.Join(outcome, reporter.err, file.Sync(), file.Close())
}

func runIssue60Group(group issue60Group, reporter *issue60Reporter) error {
	started := time.Now()
	results := make(chan error, len(group.Jobs))
	limit := make(chan struct{}, min(4, runtime.NumCPU()))
	for _, job := range group.Jobs {
		job := job
		go func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			results <- runIssue60Job(group.Name, job, reporter)
		}()
	}
	var outcome error
	for range group.Jobs {
		outcome = errors.Join(outcome, <-results)
	}
	passed := outcome == nil
	reportErr := reporter.emit(issue60Event{At: time.Now().UTC(), Group: group.Name, Kind: "group-result", Elapsed: time.Since(started), Passed: passed, Failure: errorText(outcome)})
	return errors.Join(outcome, reportErr)
}

func runIssue60Job(group string, job issue60Job, reporter *issue60Reporter) error {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), issue60JobTimeout)
	defer cancel()
	arguments := []string{"test", "-json", "-count=1", "-shuffle=off", "-timeout=" + issue60TestTimeout, "-run", job.Pattern}
	if job.Race {
		arguments = append(arguments, "-race")
	}
	arguments = append(arguments, job.Package)
	buildCache, err := os.MkdirTemp("", "ardents-issue60-go-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildCache)
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOCACHE="+buildCache)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	var outputBytes int
	readErrs := make(chan error, 2)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		var scanErr error
		for scanner.Scan() {
			raw := append([]byte(nil), scanner.Bytes()...)
			outputBytes += len(raw)
			if outputBytes > issue60MaximumOutput {
				scanErr = errors.New("issue #60 job output exceeds 32 MiB")
				cancel()
				break
			}
			if !json.Valid(raw) {
				scanErr = errors.New("go test emitted invalid JSON")
				cancel()
				break
			}
			if err := reporter.emit(issue60Event{At: time.Now().UTC(), Group: group, Job: job.Name, Package: job.Package, Kind: "go-test", Record: raw}); err != nil {
				scanErr = err
				cancel()
				break
			}
		}
		readErrs <- errors.Join(scanErr, scanner.Err())
	}()
	stderrBody, stderrErr := io.ReadAll(io.LimitReader(stderr, issue60MaximumOutput+1))
	readErr := <-readErrs
	waitErr := command.Wait()
	if len(stderrBody) > issue60MaximumOutput {
		stderrErr = errors.Join(stderrErr, errors.New("issue #60 stderr exceeds 32 MiB"))
	}
	if ctx.Err() != nil {
		waitErr = errors.Join(waitErr, fmt.Errorf("job deadline: %w", ctx.Err()))
	}
	if text := strings.TrimSpace(string(stderrBody)); text != "" {
		if emitErr := reporter.emit(issue60Event{At: time.Now().UTC(), Group: group, Job: job.Name, Package: job.Package, Kind: "job-stderr", Failure: text}); emitErr != nil {
			stderrErr = errors.Join(stderrErr, emitErr)
		}
		if waitErr != nil {
			stderrErr = errors.Join(stderrErr, errors.New(text))
		}
	}
	outcome := errors.Join(waitErr, readErr, stderrErr)
	reportErr := reporter.emit(issue60Event{At: time.Now().UTC(), Group: group, Job: job.Name, Package: job.Package, Kind: "job-result", Elapsed: time.Since(started), Passed: outcome == nil, Failure: errorText(outcome)})
	return errors.Join(outcome, reportErr)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	parts := strings.Split(err.Error(), "\n")
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}
