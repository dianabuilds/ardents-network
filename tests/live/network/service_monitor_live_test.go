//go:build live

package network_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type liveProgressPoint struct {
	at       time.Time
	received uint32
}

type liveResourcePoint struct {
	at                               time.Time
	clientCPU, publisherCPU          float64
	clientRSS, publisherRSS          float64
	clientCarrier                    float64
	publisherCarrier                 float64
	clientReceived, clientSent       float64
	publisherReceived, publisherSent float64
}

type liveTransferMonitor struct {
	started, finished time.Time
	progress          []liveProgressPoint
	resources         []liveResourcePoint
}

func monitorLiveTransfer(t *testing.T, ctx context.Context, compose composeCall, expected uint32,
	routeStarted time.Time, receiver string) liveTransferMonitor {
	t.Helper()
	services := []string{"client-service", "client-app", "client", "publisher-service", "publisher-app", "publisher"}
	identities := make(map[string]string, len(services))
	for _, service := range services {
		output, err := compose(ctx, "ps", "-q", service)
		if err != nil || strings.TrimSpace(string(output)) == "" {
			t.Fatalf("resolve %s container: %v\n%s", service, err, output)
		}
		identities[service] = strings.TrimSpace(string(output))
	}
	result := liveTransferMonitor{started: routeStarted}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var received uint32
	for received < expected {
		select {
		case <-ctx.Done():
			t.Fatalf("monitor sustained live transfer: %v", ctx.Err())
		case at := <-ticker.C:
			output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "--tail", "64", receiver)
			if err != nil {
				t.Fatalf("read sustained live progress: %v\n%s", err, output)
			}
			if current := latestLiveProgress(output); current > received {
				received = current
			}
			result.progress = append(result.progress, liveProgressPoint{at: at, received: received})
			result.resources = append(result.resources, readLiveResources(t, ctx, identities))
		}
	}
	result.finished = time.Now()
	return result
}

func directSucceeded(output []byte, expected uint32) bool {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var result struct {
			Schema        string `json:"schema"`
			Terminal      string `json:"terminal"`
			ReceivedBytes uint32 `json:"received_bytes"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &result) == nil && result.Schema == "ardents-h3-stream-application-v1" &&
			result.Terminal == "success" && result.ReceivedBytes == expected {
			return true
		}
	}
	return false
}

func latestLiveProgress(output []byte) uint32 {
	var latest uint32
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var progress struct {
			Schema        string `json:"schema"`
			ReceivedBytes uint32 `json:"received_bytes"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &progress) == nil && progress.Schema == "ardents-stream-progress-v1" &&
			progress.ReceivedBytes > latest {
			latest = progress.ReceivedBytes
		}
	}
	return latest
}

func readLiveResources(t *testing.T, ctx context.Context, identities map[string]string) liveResourcePoint {
	t.Helper()
	arguments := []string{"stats", "--no-stream", "--format", "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}|{{.NetIO}}"}
	for _, service := range []string{"client-service", "client-app", "client", "publisher-service", "publisher-app", "publisher"} {
		arguments = append(arguments, identities[service])
	}
	output, err := dockerOutput(ctx, arguments...)
	if err != nil {
		t.Fatalf("sample sustained live resources: %v\n%s", err, output)
	}
	type sample struct{ cpu, rss, received, sent float64 }
	values := make(map[string]sample, len(identities))
	matched := make(map[string]bool, len(identities))
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 4 {
			t.Fatalf("invalid Docker resource sample: %q", line)
		}
		cpu, cpuErr := strconv.ParseFloat(strings.TrimSuffix(fields[1], "%"), 64)
		rss, rssErr := liveQuantity(strings.TrimSpace(strings.Split(fields[2], "/")[0]))
		netFields := strings.Split(fields[3], "/")
		if cpuErr != nil || rssErr != nil || len(netFields) != 2 {
			t.Fatalf("invalid Docker resource values: %q", line)
		}
		received, receiveErr := liveQuantity(strings.TrimSpace(netFields[0]))
		sent, sendErr := liveQuantity(strings.TrimSpace(netFields[1]))
		if receiveErr != nil || sendErr != nil {
			t.Fatalf("invalid Docker carrier values: %q", line)
		}
		for service, identity := range identities {
			if strings.HasPrefix(identity, fields[0]) || strings.HasPrefix(fields[0], identity) {
				values[service] = sample{cpu: cpu, rss: rss, received: received, sent: sent}
				matched[service] = true
			}
		}
	}
	for service := range identities {
		if !matched[service] {
			t.Fatalf("Docker resource sample omitted %s", service)
		}
	}
	client := []string{"client-service", "client-app", "client"}
	publisher := []string{"publisher-service", "publisher-app", "publisher"}
	result := liveResourcePoint{at: time.Now()}
	for _, service := range client {
		result.clientCPU += values[service].cpu
		result.clientRSS += values[service].rss
	}
	for _, service := range publisher {
		result.publisherCPU += values[service].cpu
		result.publisherRSS += values[service].rss
	}
	result.clientReceived, result.clientSent = values["client"].received, values["client"].sent
	result.publisherReceived, result.publisherSent = values["publisher"].received, values["publisher"].sent
	result.clientCarrier = result.clientReceived + result.clientSent
	result.publisherCarrier = result.publisherReceived + result.publisherSent
	return result
}

func liveQuantity(value string) (float64, error) {
	units := []struct {
		suffix string
		scale  float64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			return parsed * unit.scale, err
		}
	}
	return 0, fmt.Errorf("unsupported Docker quantity %q", value)
}

func (value liveTransferMonitor) assert(t *testing.T, logical uint32, directGoodput float64) {
	t.Helper()
	if value.finished.Sub(value.started) < 10*time.Minute {
		t.Fatalf("sustained impaired run was shorter than ten minutes: %s", value.finished.Sub(value.started))
	}
	var zeroSince time.Time
	for index, point := range value.progress {
		if index == 0 || point.received > value.progress[index-1].received {
			zeroSince = point.at
		}
		if point.at.Sub(zeroSince) > 5*time.Second {
			t.Fatalf("impaired stream delivered no bytes for %s", point.at.Sub(zeroSince))
		}
	}
	var windows []float64
	for index, point := range value.progress {
		for earlier := index - 1; earlier >= 0; earlier-- {
			duration := point.at.Sub(value.progress[earlier].at)
			if duration >= 60*time.Second {
				bits := float64(point.received-value.progress[earlier].received) * 8
				windows = append(windows, bits/duration.Seconds()/1e6)
				break
			}
		}
	}
	threshold := math.Min(2, directGoodput*.25)
	if len(windows) == 0 || percentile(windows, 0.05) < threshold {
		t.Fatalf("p05 60-second impaired goodput is below min(2 Mbit/s, 25%% direct): route %.3f direct %.3f threshold %.3f",
			percentile(windows, 0.05), directGoodput, threshold)
	}
	clientCPU, publisherCPU := resourceSeries(value.resources, func(point liveResourcePoint) (float64, float64) {
		return point.clientCPU, point.publisherCPU
	})
	clientRSS, publisherRSS := resourceSeries(value.resources, func(point liveResourcePoint) (float64, float64) {
		return point.clientRSS, point.publisherRSS
	})
	if mean(clientCPU) > 50 || mean(publisherCPU) > 50 || percentile(clientCPU, .95) > 100 || percentile(publisherCPU, .95) > 100 {
		t.Fatalf("endpoint tree CPU exceeds its bound: client mean/p95 %.2f/%.2f publisher %.2f/%.2f",
			mean(clientCPU), percentile(clientCPU, .95), mean(publisherCPU), percentile(publisherCPU, .95))
	}
	if percentile(clientRSS, .95) > 512<<20 || percentile(publisherRSS, .95) > 512<<20 {
		t.Fatalf("endpoint tree p95 RSS exceeds 512 MiB: client %.0f publisher %.0f",
			percentile(clientRSS, .95), percentile(publisherRSS, .95))
	}
	carrierRates := make([][]float64, 4)
	for index := 1; index < len(value.resources); index++ {
		previous, current := value.resources[index-1], value.resources[index]
		seconds := current.at.Sub(previous.at).Seconds()
		if seconds <= 0 {
			continue
		}
		for direction, pair := range [][2]float64{{previous.clientReceived, current.clientReceived},
			{previous.clientSent, current.clientSent}, {previous.publisherReceived, current.publisherReceived},
			{previous.publisherSent, current.publisherSent}} {
			if pair[1] >= pair[0] {
				carrierRates[direction] = append(carrierRates[direction], (pair[1]-pair[0])*8/seconds/1e6)
			}
		}
	}
	for direction, rates := range carrierRates {
		if len(rates) == 0 || percentile(rates, .95) > 25 {
			t.Fatalf("physical direction %d p95 one-second carrier bitrate exceeds 25 Mbit/s: %.3f", direction, percentile(rates, .95))
		}
	}
	last := value.resources[len(value.resources)-1]
	if last.clientCarrier/float64(logical) > 2 || last.publisherCarrier/float64(logical) > 2 {
		t.Fatalf("endpoint carrier ratio exceeds 2.0: client %.3f publisher %.3f",
			last.clientCarrier/float64(logical), last.publisherCarrier/float64(logical))
	}
}

func resourceSeries(points []liveResourcePoint, selectValues func(liveResourcePoint) (float64, float64)) ([]float64, []float64) {
	left, right := make([]float64, 0, len(points)), make([]float64, 0, len(points))
	for _, point := range points {
		first, second := selectValues(point)
		left, right = append(left, first), append(right, second)
	}
	return left, right
}

func percentile(values []float64, fraction float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	index := int(math.Ceil(fraction*float64(len(ordered)))) - 1
	if index < 0 {
		index = 0
	}
	return ordered[index]
}

func mean(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
