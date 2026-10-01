//go:build linux

package main

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

const (
	qualificationOwnerSlice        = "ardents-qualification-owner.slice"
	qualificationOwnerControlGroup = "/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice"
)

type ownerSliceResultInput struct {
	Host, Unit, CPUQuota, CPUMax string
	MemoryMax                    uint64
	Receipt                      []string
	Samples                      []nodeOwnerSampleInput
}

type ownerSliceEvidence struct {
	Host, Unit, CPUQuota, CPUMax string
	MemoryMax                    uint64
	Samples                      int
	P95MemoryBytes               uint64
	MeanCPUPercent               float64
	TxBytes, RxBytes             uint64
}

func evaluateOwnerSlices(inputs []ownerSliceResultInput, owners map[streamqualification.Role]ownerNetworkVerdict) ([]ownerSliceEvidence, []streamqualification.Criterion) {
	evidence := make([]ownerSliceEvidence, 0, 2)
	criteria := make([]streamqualification.Criterion, 0, 8)
	seen := make(map[string]bool, 2)
	for _, input := range inputs {
		role := streamqualification.ReaderRole
		memoryLimit, cpuLimit := uint64(512<<20), float64(50)
		quota, cpuMax := "50%", "50000 100000"
		if input.Host == "publisher" {
			role, memoryLimit, cpuLimit = streamqualification.PublisherRole, 1<<30, 100
			quota, cpuMax = "100%", "100000 100000"
		}
		owner, present := owners[role]
		installed := (input.Host == "reader" || input.Host == "publisher") && !seen[input.Host] &&
			input.Unit == qualificationOwnerSlice && input.CPUQuota == quota && input.CPUMax == cpuMax &&
			input.MemoryMax == memoryLimit && owner.Started.Before(owner.Stopped) &&
			receiptHas(input.Receipt, "ActiveState", "active") &&
			receiptHas(input.Receipt, "ControlGroup", qualificationOwnerControlGroup) &&
			receiptHas(input.Receipt, "CPU_MAX", cpuMax) &&
			receiptHas(input.Receipt, "MEMORY_MAX", strconv.FormatUint(memoryLimit, 10)) &&
			receiptHas(input.Receipt, "IPAccounting", "yes")
		seen[input.Host] = true
		observed, complete := evaluateOwnerSliceWindow(input, owner.Started, owner.Stopped)
		evidence = append(evidence, observed)
		criteria = append(criteria,
			streamqualification.Criterion{Name: input.Host + "-whole-owner-slice-installed", Observed: boolNumber(installed), Relation: "=", Bound: 1, Passed: installed},
			streamqualification.Criterion{Name: input.Host + "-whole-owner-slice-samples", Observed: float64(observed.Samples), Relation: ">=", Bound: 598, Passed: present && complete && observed.Samples >= 598},
			streamqualification.Criterion{Name: input.Host + "-whole-owner-slice-RSS-bytes", Observed: float64(observed.P95MemoryBytes), Relation: "<=", Bound: float64(memoryLimit), Passed: installed && complete && observed.P95MemoryBytes <= memoryLimit},
			streamqualification.Criterion{Name: input.Host + "-whole-owner-slice-CPU-percent-one-core", Observed: observed.MeanCPUPercent, Relation: "<=", Bound: cpuLimit, Passed: installed && complete && observed.MeanCPUPercent <= cpuLimit},
		)
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].Host < evidence[j].Host })
	completeSet := len(inputs) == 2 && seen["reader"] && seen["publisher"]
	criteria = append(criteria, streamqualification.Criterion{Name: "whole-owner-slice-set", Observed: float64(len(seen)), Relation: "=", Bound: 2, Passed: completeSet})
	return evidence, criteria
}

func evaluateOwnerSliceWindow(input ownerSliceResultInput, started, stopped time.Time) (ownerSliceEvidence, bool) {
	result := ownerSliceEvidence{Host: input.Host, Unit: input.Unit, CPUQuota: input.CPUQuota, CPUMax: input.CPUMax, MemoryMax: input.MemoryMax}
	if len(input.Samples) == 0 || started.IsZero() || !started.Before(stopped) {
		return result, false
	}
	first, last := -1, -1
	for index, sample := range input.Samples {
		if !sample.At.After(started) {
			first = index
		}
		if last < 0 && !sample.At.Before(stopped) {
			last = index
		}
	}
	if first < 0 || last <= first {
		return result, false
	}
	window := input.Samples[first : last+1]
	result.Samples = len(window)
	memory := make([]uint64, 0, len(window))
	complete := len(window) >= 598
	for index, sample := range window {
		memory = append(memory, sample.MemoryCurrent)
		if sample.At.IsZero() || sample.MemoryCurrent == 0 || index > 0 &&
			(sample.At.Sub(window[index-1].At) <= 0 || sample.At.Sub(window[index-1].At) > 1500*time.Millisecond ||
				sample.CPUUsageNSec < window[index-1].CPUUsageNSec || sample.IPIngressBytes < window[index-1].IPIngressBytes ||
				sample.IPEgressBytes < window[index-1].IPEgressBytes) {
			complete = false
		}
	}
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	result.P95MemoryBytes = memory[(95*len(memory)+99)/100-1]
	begin, end := window[0], window[len(window)-1]
	seconds := end.At.Sub(begin.At).Seconds()
	if seconds < 597 || end.CPUUsageNSec < begin.CPUUsageNSec ||
		end.IPIngressBytes < begin.IPIngressBytes || end.IPEgressBytes < begin.IPEgressBytes {
		return result, false
	}
	result.MeanCPUPercent = float64(end.CPUUsageNSec-begin.CPUUsageNSec) / seconds / 10_000_000
	result.TxBytes, result.RxBytes = end.IPEgressBytes-begin.IPEgressBytes, end.IPIngressBytes-begin.IPIngressBytes
	complete = complete && !math.IsNaN(result.MeanCPUPercent) && !math.IsInf(result.MeanCPUPercent, 0) &&
		result.TxBytes <= math.MaxUint64-result.RxBytes && result.TxBytes+result.RxBytes > 0
	return result, complete
}

func receiptHas(lines []string, name, value string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == name+"="+value {
			return true
		}
	}
	return false
}
