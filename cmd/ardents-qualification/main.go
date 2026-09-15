//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

type qualificationParticipantResult struct {
	index        int
	config       endpoint.StreamQualificationConfig
	report       streamqualification.Report
	criteria     []streamqualification.Criterion
	measurements *resourceMeasurements
	err          error
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) (outcome error) {
	if len(arguments) > 0 && arguments[0] == "preflight" {
		return preflight(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "verify-pair" {
		return verifyPair(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "verify-network-manifest" {
		return verifyNetworkManifest(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "verify-net14v" {
		return verifyNET14V(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "verify-failed-net14v" {
		return verifyFailedNET14V(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "verify-run" {
		return verifyCompletedRun(arguments[1:], output)
	}
	if len(arguments) != 1 {
		return errors.New("usage: ardents-qualification <local-plan.json> | preflight <local-plan.json> | verify-run <runner-jsonl> | verify-pair <reader-jsonl> <publisher-jsonl> <network-manifest.json> <relay-results.json> <node-results.json> <node-inventory-sha256> <cleanup-results.json> | verify-network-manifest <manifest.json> | verify-net14v <baseline-manifest.json> <episode-manifest.json> <baseline-verdict.json> <episode-verdict.json> <recovery-evidence.jsonl> [recovery-evidence.jsonl] | verify-failed-net14v <baseline-manifest.json> <episode-manifest.json> <baseline-verdict.json> <failed-relay-results.json> <recovery-evidence.jsonl> [recovery-evidence.jsonl]")
	}
	journal := newEvidenceJournal(output)
	emit := journal.emit
	defer func() { outcome = errors.Join(outcome, journal.finish(outcome)) }()
	endpointArtifact, err := verifyQualificationEndpointArtifact(arguments[0])
	if err != nil {
		return err
	}
	file, err := os.Open(arguments[0])
	if err != nil {
		return err
	}
	planDigest := sha256.New()
	plan, err := decodePlan(io.TeeReader(file, planDigest))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err := os.Open(executable)
	if err != nil {
		return err
	}
	digest := sha256.New()
	_, hashErr := io.Copy(digest, binary)
	if err := errors.Join(hashErr, binary.Close()); err != nil {
		return err
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("qualification build identity unavailable")
	}
	environment, err := readCandidateEnvironment(ctx)
	if err != nil {
		return err
	}
	if err := emit(struct {
		Participants                  int
		PlanSHA256                    string
		Mode                          string
		EndpointArtifact              qualificationEndpointArtifact
		Environment                   candidateEnvironment
		Kind, BinarySHA256, GoVersion string
		Build                         *debug.BuildInfo
	}{len(plan.Configs), hex.EncodeToString(planDigest.Sum(nil)), plan.Mode, endpointArtifact, environment, "candidate", hex.EncodeToString(digest.Sum(nil)), runtime.Version(), build}); err != nil {
		return err
	}
	configs := plan.Configs
	if plan.Mode == "net32-idle" {
		return runNET32Idle(ctx, configs[0], emit)
	}
	group, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan qualificationParticipantResult, len(configs))
	ownerMeasurements, err := endpoint.NewStreamQualificationMeasurements(len(configs))
	if err != nil {
		return err
	}
	for index, config := range configs {
		index, config := index, config
		config.Measurements = ownerMeasurements
		measurements := &resourceMeasurements{}
		config.Observe = func(eventCtx context.Context, event endpoint.StreamQualificationEvent) error {
			measurements.observe(event)
			if err := eventCtx.Err(); err != nil {
				return err
			}
			return emit(struct {
				Participant int
				Event       endpoint.StreamQualificationEvent
			}{index, event})
		}
		config.Participant.Observe = func(eventCtx context.Context, event endpoint.TextParticipantEvent) error {
			if err := eventCtx.Err(); err != nil {
				return err
			}
			return emit(struct {
				Participant int
				Event       endpoint.TextParticipantEvent
			}{index, event})
		}
		go func() {
			report, err := endpoint.RunStreamQualification(group, config)
			criteria := streamqualification.EvaluateConditionWorkload(report, config.Role, config.Profile, config.Condition)
			if !streamqualification.CriteriaPassed(criteria) {
				err = errors.Join(err, errors.New("qualification workload criteria failed"))
			}
			if err != nil {
				cancel()
			}
			results <- qualificationParticipantResult{index, config, report, criteria, measurements, err}
		}()
	}
	completed := make([]qualificationParticipantResult, len(configs))
	for range configs {
		result := <-results
		completed[result.index] = result
	}
	reports := make([]streamqualification.Report, len(completed))
	measurements := make([]*resourceMeasurements, len(completed))
	for index, result := range completed {
		reports[index], measurements[index] = result.report, result.measurements
	}
	ownerNetwork, ownerCriteria := evaluateOwnerNetwork(measurements, reports, configs[0].Role, configs[0].Profile, configs[0].Condition)
	resourceCriteria := completed[0].measurements.evaluate(completed[0].report, configs[0].Role)
	for _, criterion := range resourceCriteria {
		switch criterion.Name {
		case "p95-owner-RSS-bytes":
			ownerNetwork.P95RSSBytes = uint64(criterion.Observed)
		case "mean-owner-CPU-percent-one-core":
			ownerNetwork.MeanCPUPercent = criterion.Observed
		}
	}
	ownerCriteria = append(ownerCriteria, resourceCriteria...)
	ownerErr := error(nil)
	if !streamqualification.CriteriaPassed(ownerCriteria) {
		ownerErr = errors.New("qualification owner network criteria failed")
	}
	for _, result := range completed {
		criteria := append(result.criteria, ownerCriteria...)
		participantErr := errors.Join(result.err, ownerErr)
		failure := ""
		if participantErr != nil {
			failure = participantErr.Error()
		}
		writeErr := emit(struct {
			Kind         string
			Seed         string
			Role         streamqualification.Role
			Profile      streamqualification.Profile
			Condition    streamqualification.NetworkCondition
			ReaderIndex  int
			Participant  int
			Report       any
			OwnerNetwork ownerNetworkVerdict
			Criteria     []streamqualification.Criterion
			Failure      string
		}{"result", hex.EncodeToString(result.config.Seed[:]), result.config.Role, result.config.Profile, result.config.Condition, result.config.ReaderIndex, result.index, result.report, ownerNetwork, criteria, failure})
		outcome = errors.Join(outcome, participantErr, writeErr)
	}
	return outcome
}
