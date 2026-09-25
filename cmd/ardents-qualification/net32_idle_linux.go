//go:build linux

package main

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/qualification"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type net32IdleVerdict struct {
	Kind                   string
	Observation            string
	Report                 endpoint.StreamQualificationIdleReport
	StateBytes             uint64
	StateFiles             uint64
	InterfaceTx            uint64
	InterfaceRx            uint64
	ObservedAggregateBytes uint64
	Projected24HourBytes   uint64
	HostingLedgerDelta     uint64
	Criteria               []streamqualification.Criterion
	Failure                string
}

func runNET32Idle(ctx context.Context, config endpoint.StreamQualificationConfig, emit func(any) error) (outcome error) {
	measurements, err := qualification.NewMeasurements(1)
	if err != nil {
		return err
	}
	series := &resourceMeasurements{}
	config.Measurements = measurements
	config.Observe = func(eventCtx context.Context, event endpoint.StreamQualificationEvent) error {
		series.observe(event)
		if err := eventCtx.Err(); err != nil {
			return err
		}
		return emit(struct {
			Participant int
			Event       endpoint.StreamQualificationEvent
		}{0, event})
	}
	config.Participant.Observe = func(eventCtx context.Context, event endpoint.TextParticipantEvent) error {
		if err := eventCtx.Err(); err != nil {
			return err
		}
		return emit(struct {
			Participant int
			Event       endpoint.TextParticipantEvent
		}{0, event})
	}
	stateBytes, stateFiles, stateErr := boundedStateProfile(config.Participant.Network.Root)
	report, runErr := endpoint.RunStreamQualificationIdle(ctx, config)
	verdict, criteria := evaluateNET32Idle(report, series, stateBytes, stateFiles)
	criteria = append(criteria, streamqualification.Criterion{Name: "state-profile-bytes", Observed: float64(stateBytes), Relation: "<=", Bound: 64 << 10, Passed: stateErr == nil && stateBytes <= 64<<10})
	verdict.StateBytes, verdict.StateFiles, verdict.Criteria = stateBytes, stateFiles, criteria
	if stateErr != nil || !streamqualification.CriteriaPassed(criteria) {
		runErr = errors.Join(runErr, stateErr, errors.New("NET-32 short projection criteria failed"))
	}
	verdict.Failure = errorText(runErr)
	writeErr := emit(struct {
		Kind        string
		Participant int
		Result      net32IdleVerdict
	}{"result", 0, verdict})
	return errors.Join(runErr, writeErr)
}

func evaluateNET32Idle(report endpoint.StreamQualificationIdleReport, series *resourceMeasurements, stateBytes, stateFiles uint64) (net32IdleVerdict, []streamqualification.Criterion) {
	verdict := net32IdleVerdict{Kind: "net32-short-projection", Observation: "ten-minute observation with an upper 24-hour projection; not a 24-hour observed run", Report: report, StateBytes: stateBytes, StateFiles: stateFiles}
	criteria := make([]streamqualification.Criterion, 0, 8)
	add := func(name string, observed float64, relation string, bound float64, passed bool) {
		criteria = append(criteria, streamqualification.Criterion{Name: name, Observed: observed, Relation: relation, Bound: bound, Passed: passed})
	}
	complete := report.Failure == "" && !report.Started.IsZero() && !report.Stopped.IsZero() && report.MeasuredDuration >= 10*time.Minute && series != nil && len(series.samples) >= 600
	add("idle-observation-seconds", report.MeasuredDuration.Seconds(), ">=", 600, complete)
	add("one-second-owner-sampling", float64(len(series.samples)), ">=", 600, complete)
	if !complete {
		return verdict, criteria
	}
	first, last := series.samples[0], series.samples[len(series.samples)-1]
	interfaceComplete := !first.host.At.After(report.Started) && !last.host.At.Before(report.Stopped) && sameHostingPolicy(first.host.Policy, last.host.Policy) && first.host.Boot == last.host.Boot && len(first.host.Interfaces) == len(last.host.Interfaces)
	var tx, rx uint64
	for index, before := range first.host.Interfaces {
		after := last.host.Interfaces[index]
		if before.Name != after.Name || before.Index != after.Index || after.Tx < before.Tx || after.Rx < before.Rx || after.Tx-before.Tx > math.MaxUint64-tx || after.Rx-before.Rx > math.MaxUint64-rx {
			interfaceComplete = false
			break
		}
		tx += after.Tx - before.Tx
		rx += after.Rx - before.Rx
	}
	verdict.InterfaceTx, verdict.InterfaceRx = tx, rx
	aggregateOK := tx <= math.MaxUint64-rx
	if aggregateOK {
		verdict.ObservedAggregateBytes = tx + rx
	}
	if aggregateOK && report.MeasuredDuration > 0 {
		projected := math.Ceil(float64(verdict.ObservedAggregateBytes) * (24 * time.Hour).Seconds() / report.MeasuredDuration.Seconds())
		if projected <= math.MaxUint64 {
			verdict.Projected24HourBytes = uint64(projected)
		} else {
			aggregateOK = false
		}
	}
	add("interface-counter-window", float64(len(first.host.Interfaces)), "complete", float64(len(last.host.Interfaces)), interfaceComplete)
	add("projected-24-hour-aggregate-bytes", float64(verdict.Projected24HourBytes), "<=", 1_000_000_000, interfaceComplete && aggregateOK && verdict.Projected24HourBytes <= 1_000_000_000)
	counted, countedOK := hostingCountedDelta(first.host.Policy, tx, rx)
	ledgerOK := last.host.Observation.UsedBytes >= first.host.Observation.UsedBytes
	if ledgerOK {
		verdict.HostingLedgerDelta = last.host.Observation.UsedBytes - first.host.Observation.UsedBytes
	}
	add("hosting-ledger-bytes", float64(verdict.HostingLedgerDelta), "= interface policy bytes", float64(counted), interfaceComplete && countedOK && ledgerOK && verdict.HostingLedgerDelta == counted)
	var memory []uint64
	resourceComplete := true
	for index, sample := range series.samples {
		memory = append(memory, sample.memory)
		if sample.memory == 0 || index > 0 && (sample.at <= series.samples[index-1].at || sample.at-series.samples[index-1].at > 1500*time.Millisecond || sample.cpu < series.samples[index-1].cpu) {
			resourceComplete = false
		}
	}
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	p95 := memory[(95*len(memory)+99)/100-1]
	cpu := float64(0)
	if last.at > first.at && last.cpu >= first.cpu {
		cpu = float64(last.cpu-first.cpu) / (last.at - first.at).Seconds() / 10_000
	} else {
		resourceComplete = false
	}
	add("p95-owner-RSS-bytes", float64(p95), "<=", 512<<20, resourceComplete && p95 <= 512<<20)
	add("mean-owner-CPU-percent-one-core", cpu, "<=", 1, resourceComplete && cpu <= 1)
	return verdict, criteria
}

func hostingCountedDelta(policy resource.HostingPolicy, tx, rx uint64) (uint64, bool) {
	switch policy.Directions {
	case "tx":
		return tx, true
	case "rx":
		return rx, true
	case "tx+rx":
		if tx > math.MaxUint64-rx {
			return 0, false
		}
		return tx + rx, true
	default:
		return 0, false
	}
}

func boundedStateProfile(root string) (bytes, files uint64, outcome error) {
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("State profile contains a non-regular entry")
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || uint64(info.Size()) > (64<<10)-bytes {
				return errors.New("State profile exceeds 64 KiB")
			}
			bytes += uint64(info.Size())
			files++
		}
		return nil
	})
	return bytes, files, err
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
