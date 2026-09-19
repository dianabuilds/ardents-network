//go:build linux

package main

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func sourceResourceWindow(input nodeResultInput, started, stopped time.Time) (uint64, float64, bool) {
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
	memory := make([]uint64, 0, len(window))
	for index, sample := range window {
		memory = append(memory, sample.MemoryCurrent)
		if sample.At.IsZero() || sample.MemoryCurrent == 0 || index > 0 &&
			(sample.At.Sub(window[index-1].At) <= 0 || sample.At.Sub(window[index-1].At) > 1500*time.Millisecond ||
				sample.CPUUsageNSec < window[index-1].CPUUsageNSec) {
			complete = false
		}
	}
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	p95Memory := memory[(95*len(memory)+99)/100-1]
	p95RSS, rssComplete := sourceRSSWindow(input.Journal, started, stopped)
	seconds := window[len(window)-1].At.Sub(window[0].At).Seconds()
	if seconds < 597 || window[len(window)-1].CPUUsageNSec < window[0].CPUUsageNSec {
		return max(p95Memory, p95RSS), 0, false
	}
	cpu := float64(window[len(window)-1].CPUUsageNSec-window[0].CPUUsageNSec) / seconds / 10_000_000
	return max(p95Memory, p95RSS), cpu, complete && rssComplete && !math.IsNaN(cpu) && !math.IsInf(cpu, 0)
}

func sourceRSSWindow(journal []string, started, stopped time.Time) (uint64, bool) {
	type observation struct {
		at  time.Time
		rss uint64
	}
	var samples []observation
	for _, line := range journal {
		var event struct {
			Schema   string
			Kind     string
			At       time.Time
			Resource resource.Sample
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Schema == "ardents-h3-resource-sample-v1" &&
			event.Kind == "resource-sample" {
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
		if sample.at.IsZero() || sample.rss == 0 || index > 0 &&
			(sample.at.Sub(window[index-1].at) <= 0 || sample.at.Sub(window[index-1].at) > 1500*time.Millisecond) {
			complete = false
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[(95*len(values)+99)/100-1], complete
}
