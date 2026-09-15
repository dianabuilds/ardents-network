//go:build linux

package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type nodeOwnerSampleInput struct {
	At                            time.Time
	MemoryCurrent, CPUUsageNSec   uint64
	IPIngressBytes, IPEgressBytes uint64
}

type nodeResultInput struct {
	ID, Host, PlanSHA256, InvocationID, BinarySHA256 string
	ActiveState, Result, Slice                       string
	ExecMainStatus                                   int
	Journal                                          []string
	Samples                                          []nodeOwnerSampleInput
}

type nodeOwnerEvidence struct {
	ID, Host, PlanSHA256, InvocationID string
	Samples, NodeSamples               int
	P95MemoryBytes, P95RSSBytes        uint64
	MeanCPUPercent                     float64
	TxBytes, RxBytes                   uint64
}

type sourceOwnerEvidence struct {
	ID, Host, PlanSHA256, InvocationID string
	Samples, StateSamples              int
	P95MemoryBytes, P95RSSBytes        uint64
	MeanCPUPercent                     float64
	TxBytes, RxBytes                   uint64
}

type nodeHostEvidence struct {
	Host                                     string
	Nodes, Sources                           int
	NodeP95RSSBytes, SourceP95RSSBytes       uint64
	CombinedP95RSSBytes                      uint64
	NodeMeanCPUPercent, SourceMeanCPUPercent float64
	CombinedMeanCPUPercent                   float64
}

type nodeOwnersVerdict struct {
	InventorySHA256, BinarySHA256 string
	Nodes                         []nodeOwnerEvidence
	Sources                       []sourceOwnerEvidence
	Hosts                         []nodeHostEvidence
	OwnerSlices                   []ownerSliceEvidence
}

type nodeResultsInput struct {
	InventorySHA256 string
	Nodes           []nodeResultInput
	Sources         []nodeResultInput
	OwnerSlices     []ownerSliceResultInput
}

func readNodeResults(path string, manifest qualificationNetworkManifest, inventorySHA256 string, owners map[streamqualification.Role]ownerNetworkVerdict) (nodeOwnersVerdict, []streamqualification.Criterion, error) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || len(body) > 16<<20 {
		return nodeOwnersVerdict{}, nil, errors.Join(err, errors.New("Node result input is invalid"))
	}
	var inputSet nodeResultsInput
	if err := decodeExact(body, &inputSet); err != nil {
		return nodeOwnersVerdict{}, nil, err
	}
	inputs, sources := inputSet.Nodes, inputSet.Sources
	expected, err := manifestNodeIDs(manifest)
	if err != nil {
		return nodeOwnersVerdict{}, nil, err
	}
	criteria := make([]streamqualification.Criterion, 0, len(inputs)+len(sources)+2)
	add := func(name string, observed float64, relation string, bound float64, passed bool) {
		criteria = append(criteria, streamqualification.Criterion{Name: name, Observed: observed, Relation: relation, Bound: bound, Passed: passed})
	}
	verdict := nodeOwnersVerdict{InventorySHA256: inputSet.InventorySHA256}
	binaryIdentity := ""
	seen := make(map[string]bool, len(inputs)+len(sources))
	_, inventoryErr := decodeIdentity(inputSet.InventorySHA256)
	clean := inventoryErr == nil && inputSet.InventorySHA256 == inventorySHA256 && len(inputs) == 16 && len(sources) == 2
	for _, input := range inputs {
		if !validOwnerResultIdentity(input, seen, &binaryIdentity) {
			clean = false
			continue
		}
		owner, ownerOK := evaluateNodeOwner(input)
		verdict.Nodes = append(verdict.Nodes, owner)
		add("node-owner-"+input.ID, float64(owner.Samples), ">=", 598, ownerOK)
	}
	for id := range expected {
		clean = clean && seen[id]
	}
	sourceHosts := make(map[string]bool, 2)
	for _, input := range sources {
		if !validOwnerResultIdentity(input, seen, &binaryIdentity) || sourceHosts[input.Host] {
			clean = false
			continue
		}
		sourceHosts[input.Host] = true
		owner, ownerOK := evaluateSourceOwner(input)
		verdict.Sources = append(verdict.Sources, owner)
		add("state-source-owner-"+input.ID, float64(owner.Samples), ">=", 598, ownerOK)
	}
	clean = clean && sourceHosts["reader"] && sourceHosts["publisher"]
	sort.Slice(verdict.Nodes, func(i, j int) bool { return verdict.Nodes[i].ID < verdict.Nodes[j].ID })
	sort.Slice(verdict.Sources, func(i, j int) bool { return verdict.Sources[i].ID < verdict.Sources[j].ID })
	hosts, hostCriteria := evaluateNodeHostResources(inputs, sources, owners)
	verdict.Hosts = hosts
	criteria = append(criteria, hostCriteria...)
	ownerSlices, sliceCriteria := evaluateOwnerSlices(inputSet.OwnerSlices, owners)
	verdict.OwnerSlices = ownerSlices
	criteria = append(criteria, sliceCriteria...)
	verdict.BinarySHA256 = binaryIdentity
	add("route-node-owner-set", float64(len(verdict.Nodes)), "=", 16, clean && len(verdict.Nodes) == 16)
	add("state-source-owner-set", float64(len(verdict.Sources)), "=", 2, clean && len(verdict.Sources) == 2)
	if !streamqualification.CriteriaPassed(criteria) {
		return verdict, criteria, errors.New("Route Node and State Source owner evidence is incomplete")
	}
	return verdict, criteria, nil
}

func validOwnerResultIdentity(input nodeResultInput, seen map[string]bool, binaryIdentity *string) bool {
	if _, err := decodeIdentity(input.ID); err != nil || seen[input.ID] || input.Host != "reader" && input.Host != "publisher" || input.Slice != qualificationOwnerSlice {
		return false
	}
	seen[input.ID] = true
	if _, err := decodeIdentity(input.BinarySHA256); err != nil || *binaryIdentity != "" && *binaryIdentity != input.BinarySHA256 {
		return false
	}
	*binaryIdentity = input.BinarySHA256
	if _, err := decodeIdentity(input.PlanSHA256); err != nil {
		return false
	}
	invocation, err := hex.DecodeString(input.InvocationID)
	return err == nil && len(invocation) == 16 && hex.EncodeToString(invocation) == input.InvocationID
}
func manifestNodeIDs(manifest qualificationNetworkManifest) (map[string]bool, error) {
	ids := make(map[string]bool, 5)
	for _, path := range manifest.Paths {
		for _, segment := range path.Segments {
			for _, id := range []string{segment.From, segment.To} {
				if id == userEndpoint || id == publisherEndpoint {
					continue
				}
				if _, err := decodeIdentity(id); err != nil {
					return nil, errors.New("network manifest intermediate Node identity is invalid")
				}
				ids[id] = true
			}
		}
	}
	if len(ids) != 5 {
		return nil, errors.New("network manifest must bind five intermediate Nodes")
	}
	return ids, nil
}

func evaluateNodeOwner(input nodeResultInput) (nodeOwnerEvidence, bool) {
	owner := nodeOwnerEvidence{ID: input.ID, Host: input.Host, PlanSHA256: input.PlanSHA256, InvocationID: input.InvocationID, Samples: len(input.Samples)}
	ready, withdrawn, failed := 0, 0, false
	var rss []uint64
	var hostingPolicy *resource.HostingPolicy
	var hostingUsed uint64
	for _, line := range input.Journal {
		var event node.Event
		if json.Unmarshal([]byte(line), &event) != nil || event.Schema != "ardents-node-event-v1" {
			continue
		}
		switch {
		case event.Kind == "lifecycle" && event.State == "READY":
			ready++
		case event.Kind == "lifecycle" && event.State == "WITHDRAWN":
			withdrawn++
		case event.Kind == "lifecycle" && event.State == "FAILED":
			failed = true
		case event.Kind == "resource-sample" && event.State == "OBSERVED" && event.Resource != nil && event.Hosting != nil:
			if event.Resource.RSSBytes == 0 {
				failed = true
			}
			rss = append(rss, event.Resource.RSSBytes)
			if hostingPolicy != nil && (!sameHostingPolicy(*hostingPolicy, event.Hosting.Policy) || event.Hosting.Observation.UsedBytes < hostingUsed) {
				failed = true
			}
			policy := event.Hosting.Policy
			hostingPolicy, hostingUsed = &policy, event.Hosting.Observation.UsedBytes
			owner.NodeSamples++
		}
	}
	complete := input.ActiveState == "inactive" && input.Result == "success" && input.ExecMainStatus == 0 && ready == 1 && withdrawn == 1 && !failed && owner.NodeSamples >= 598 && len(input.Samples) >= 598 && len(rss) >= 598
	if len(rss) > 0 {
		sort.Slice(rss, func(i, j int) bool { return rss[i] < rss[j] })
		owner.P95RSSBytes = rss[(95*len(rss)+99)/100-1]
	}
	if len(input.Samples) == 0 {
		return owner, false
	}
	memory := make([]uint64, 0, len(input.Samples))
	for index, sample := range input.Samples {
		memory = append(memory, sample.MemoryCurrent)
		if sample.At.IsZero() || sample.MemoryCurrent == 0 || index > 0 && (sample.At.Sub(input.Samples[index-1].At) <= 0 || sample.At.Sub(input.Samples[index-1].At) > 1500*time.Millisecond || sample.CPUUsageNSec < input.Samples[index-1].CPUUsageNSec || sample.IPIngressBytes < input.Samples[index-1].IPIngressBytes || sample.IPEgressBytes < input.Samples[index-1].IPEgressBytes) {
			complete = false
		}
	}
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	owner.P95MemoryBytes = memory[(95*len(memory)+99)/100-1]
	memoryBound := uint64(1 << 30)
	if input.Host == "reader" {
		memoryBound = 512 << 20
	}
	complete = complete && owner.P95MemoryBytes <= memoryBound
	first, last := input.Samples[0], input.Samples[len(input.Samples)-1]
	owner.TxBytes, owner.RxBytes = last.IPEgressBytes-first.IPEgressBytes, last.IPIngressBytes-first.IPIngressBytes
	if owner.TxBytes > math.MaxUint64-owner.RxBytes || owner.TxBytes+owner.RxBytes == 0 {
		complete = false
	}
	seconds := last.At.Sub(first.At).Seconds()
	if seconds < 597 || last.CPUUsageNSec < first.CPUUsageNSec {
		complete = false
	} else {
		owner.MeanCPUPercent = float64(last.CPUUsageNSec-first.CPUUsageNSec) / seconds / 10_000_000
		complete = complete && !math.IsNaN(owner.MeanCPUPercent) && owner.MeanCPUPercent <= 100
	}
	return owner, complete
}

func evaluateSourceOwner(input nodeResultInput) (sourceOwnerEvidence, bool) {
	owner := sourceOwnerEvidence{ID: input.ID, Host: input.Host, PlanSHA256: input.PlanSHA256, InvocationID: input.InvocationID, Samples: len(input.Samples)}
	ready, failed := 0, false
	var rss []uint64
	for _, line := range input.Journal {
		var event struct {
			Schema   string
			Kind     string
			Resource resource.Sample
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Schema == "ardents-source-event-v1" && event.Kind == "source-ready" {
			ready++
		}
		if event.Schema == "ardents-h3-resource-sample-v1" && event.Kind == "resource-sample" {
			owner.StateSamples++
			if event.Resource.RSSBytes == 0 {
				failed = true
			}
			rss = append(rss, event.Resource.RSSBytes)
		}
	}
	complete := input.ActiveState == "inactive" && input.Result == "success" && input.ExecMainStatus == 0 &&
		ready == 1 && !failed && owner.StateSamples >= 598 && len(input.Samples) >= 598 && len(rss) >= 598
	if len(rss) > 0 {
		sort.Slice(rss, func(i, j int) bool { return rss[i] < rss[j] })
		owner.P95RSSBytes = rss[(95*len(rss)+99)/100-1]
	}
	if len(input.Samples) == 0 {
		return owner, false
	}
	memory := make([]uint64, 0, len(input.Samples))
	for index, sample := range input.Samples {
		memory = append(memory, sample.MemoryCurrent)
		if sample.At.IsZero() || sample.MemoryCurrent == 0 || index > 0 &&
			(sample.At.Sub(input.Samples[index-1].At) <= 0 || sample.At.Sub(input.Samples[index-1].At) > 1500*time.Millisecond ||
				sample.CPUUsageNSec < input.Samples[index-1].CPUUsageNSec || sample.IPIngressBytes < input.Samples[index-1].IPIngressBytes ||
				sample.IPEgressBytes < input.Samples[index-1].IPEgressBytes) {
			complete = false
		}
	}
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	owner.P95MemoryBytes = memory[(95*len(memory)+99)/100-1]
	first, last := input.Samples[0], input.Samples[len(input.Samples)-1]
	seconds := last.At.Sub(first.At).Seconds()
	if seconds < 597 || last.CPUUsageNSec < first.CPUUsageNSec ||
		last.IPEgressBytes < first.IPEgressBytes || last.IPIngressBytes < first.IPIngressBytes {
		return owner, false
	}
	owner.MeanCPUPercent = float64(last.CPUUsageNSec-first.CPUUsageNSec) / seconds / 10_000_000
	owner.TxBytes, owner.RxBytes = last.IPEgressBytes-first.IPEgressBytes, last.IPIngressBytes-first.IPIngressBytes
	complete = complete && !math.IsNaN(owner.MeanCPUPercent) && !math.IsInf(owner.MeanCPUPercent, 0) &&
		owner.TxBytes <= math.MaxUint64-owner.RxBytes && owner.TxBytes+owner.RxBytes > 0
	return owner, complete
}
func evaluateNodeHostResources(inputs, sources []nodeResultInput, owners map[streamqualification.Role]ownerNetworkVerdict) ([]nodeHostEvidence, []streamqualification.Criterion) {
	var evidence []nodeHostEvidence
	var criteria []streamqualification.Criterion
	for _, host := range []string{"reader", "publisher"} {
		role := streamqualification.ReaderRole
		memoryLimit, cpuLimit := uint64(512<<20), float64(50)
		if host == "publisher" {
			role, memoryLimit, cpuLimit = streamqualification.PublisherRole, 1<<30, 100
		}
		owner, ownerPresent := owners[role]
		observed := nodeHostEvidence{Host: host}
		complete := ownerPresent && owner.P95RSSBytes > 0 && !owner.Started.IsZero() && owner.Started.Before(owner.Stopped)
		for _, input := range inputs {
			if input.Host != host {
				continue
			}
			observed.Nodes++
			memory, cpu, ok := nodeResourceWindow(input, owner.Started, owner.Stopped)
			if !ok || memory > math.MaxUint64-observed.NodeP95RSSBytes {
				complete = false
				continue
			}
			observed.NodeP95RSSBytes += memory
			observed.NodeMeanCPUPercent += cpu
		}
		for _, input := range sources {
			if input.Host != host {
				continue
			}
			observed.Sources++
			memory, cpu, ok := sourceResourceWindow(input, owner.Started, owner.Stopped)
			if !ok || memory > math.MaxUint64-observed.SourceP95RSSBytes {
				complete = false
				continue
			}
			observed.SourceP95RSSBytes += memory
			observed.SourceMeanCPUPercent += cpu
		}
		combinedSupport := observed.NodeP95RSSBytes + observed.SourceP95RSSBytes
		if observed.Nodes == 0 || observed.Sources != 1 || combinedSupport < observed.NodeP95RSSBytes ||
			owner.P95RSSBytes > math.MaxUint64-combinedSupport {
			complete = false
		} else {
			observed.CombinedP95RSSBytes = owner.P95RSSBytes + combinedSupport
		}
		observed.CombinedMeanCPUPercent = owner.MeanCPUPercent + observed.NodeMeanCPUPercent + observed.SourceMeanCPUPercent
		complete = complete && !math.IsNaN(observed.CombinedMeanCPUPercent) && !math.IsInf(observed.CombinedMeanCPUPercent, 0)
		criteria = append(criteria,
			streamqualification.Criterion{Name: host + "-complete-owner-RSS-bytes", Observed: float64(observed.CombinedP95RSSBytes), Relation: "<=", Bound: float64(memoryLimit), Passed: complete && observed.CombinedP95RSSBytes <= memoryLimit},
			streamqualification.Criterion{Name: host + "-complete-owner-CPU-percent-one-core", Observed: observed.CombinedMeanCPUPercent, Relation: "<=", Bound: cpuLimit, Passed: complete && observed.CombinedMeanCPUPercent <= cpuLimit},
		)
		evidence = append(evidence, observed)
	}
	return evidence, criteria
}

func nodeResourceWindow(input nodeResultInput, started, stopped time.Time) (uint64, float64, bool) {
	samples := input.Samples
	if len(samples) == 0 || started.IsZero() || !started.Before(stopped) {
		return 0, 0, false
	}
	first, last := -1, -1
	for index, sample := range samples {
		if !sample.At.After(started) {
			first = index
		}
		if last < 0 && !sample.At.Before(stopped) {
			last = index
		}
	}
	if first < 0 || last <= first || last-first+1 < 598 {
		return 0, 0, false
	}
	window := samples[first : last+1]
	complete := true
	for index, sample := range window {
		if sample.At.IsZero() || sample.MemoryCurrent == 0 || index > 0 && (sample.At.Sub(window[index-1].At) <= 0 || sample.At.Sub(window[index-1].At) > 1500*time.Millisecond || sample.CPUUsageNSec < window[index-1].CPUUsageNSec) {
			complete = false
		}
	}
	p95, rssComplete := nodeRSSWindow(input.Journal, started, stopped)
	seconds := window[len(window)-1].At.Sub(window[0].At).Seconds()
	if seconds < 597 || window[len(window)-1].CPUUsageNSec < window[0].CPUUsageNSec {
		return p95, 0, false
	}
	cpu := float64(window[len(window)-1].CPUUsageNSec-window[0].CPUUsageNSec) / seconds / 10_000_000
	return p95, cpu, complete && rssComplete && !math.IsNaN(cpu) && !math.IsInf(cpu, 0)
}

func nodeRSSWindow(journal []string, started, stopped time.Time) (uint64, bool) {
	type observation struct {
		at  time.Time
		rss uint64
	}
	var samples []observation
	for _, line := range journal {
		var event node.Event
		if json.Unmarshal([]byte(line), &event) == nil && event.Schema == "ardents-node-event-v1" && event.Kind == "resource-sample" && event.State == "OBSERVED" && event.Resource != nil {
			samples = append(samples, observation{at: event.At, rss: event.Resource.RSSBytes})
		}
	}
	first, last := -1, -1
	for index, sample := range samples {
		if !sample.at.After(started) {
			first = index
		}
		if last < 0 && !sample.at.Before(stopped) {
			last = index
		}
	}
	if first < 0 || last <= first || last-first+1 < 598 {
		return 0, false
	}
	window := samples[first : last+1]
	values := make([]uint64, 0, len(window))
	complete := true
	for index, sample := range window {
		values = append(values, sample.rss)
		if sample.at.IsZero() || sample.rss == 0 || index > 0 && (sample.at.Sub(window[index-1].at) <= 0 || sample.at.Sub(window[index-1].at) > 1500*time.Millisecond) {
			complete = false
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[(95*len(values)+99)/100-1], complete
}
