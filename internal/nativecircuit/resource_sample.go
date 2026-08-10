package nativecircuit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type dockerResourceSample struct {
	ElapsedMilliseconds int64   `json:"elapsed_milliseconds"`
	Role                string  `json:"role"`
	CPUCores            float64 `json:"cpu_cores"`
	RSSBytes            uint64  `json:"rss_bytes"`
	RXBytes             uint64  `json:"rx_bytes"`
	TXBytes             uint64  `json:"tx_bytes"`
	PIDs                int     `json:"pids"`
	containerID         string
}

type resourceSampler struct {
	cancel context.CancelFunc
	done   chan resourceSampleResult
	once   sync.Once
	result resourceSampleResult
}

type resourceSampleResult struct {
	samples []dockerResourceSample
	err     error
}

func startResourceSampler(ctx context.Context, roles map[string]string) *resourceSampler {
	sampleContext, cancel := context.WithCancel(ctx)
	sampler := &resourceSampler{cancel: cancel, done: make(chan resourceSampleResult, 1)}
	go func() { sampler.done <- sampleDockerResources(sampleContext, roles) }()
	return sampler
}

func (sampler *resourceSampler) stop() ([]dockerResourceSample, error) {
	if sampler == nil {
		return nil, nil
	}
	sampler.once.Do(func() {
		sampler.cancel()
		sampler.result = <-sampler.done
	})
	return sampler.result.samples, sampler.result.err
}

func sampleDockerResources(ctx context.Context, roles map[string]string) resourceSampleResult {
	started := time.Now()
	var samples []dockerResourceSample
	for {
		batch, err := readDockerResources(ctx, roles, time.Since(started).Milliseconds())
		if err != nil && !errors.Is(err, context.Canceled) {
			return resourceSampleResult{samples: samples, err: err}
		}
		samples = append(samples, batch...)
		select {
		case <-ctx.Done():
			return resourceSampleResult{samples: samples}
		case <-time.After(time.Second):
		}
	}
}

func readDockerResources(ctx context.Context, roles map[string]string, elapsed int64) ([]dockerResourceSample, error) {
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	arguments := []string{"stats", "--no-stream", "--format", "{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.PIDs}}"}
	output, err := exec.CommandContext(ctx, "docker", append(arguments, ids...)...).Output()
	if err != nil {
		return nil, err
	}
	var samples []dockerResourceSample
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		sample, err := parseDockerResourceLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		role, found := roles[sample.containerID]
		if !found {
			return nil, errors.New("docker stats returned an unknown native container")
		}
		sample.Role, sample.ElapsedMilliseconds = role, elapsed
		samples = append(samples, sample)
	}
	if err := scanner.Err(); err != nil || len(samples) != len(roles) {
		return nil, errors.New("docker stats did not return the exact native process set")
	}
	return samples, nil
}

func parseDockerResourceLine(line string) (dockerResourceSample, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 6 {
		return dockerResourceSample{}, errors.New("docker stats line has an unexpected shape")
	}
	cpu, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
	if err != nil {
		return dockerResourceSample{}, err
	}
	memory := strings.Split(fields[3], " / ")
	network := strings.Split(fields[4], " / ")
	if len(memory) != 2 || len(network) != 2 {
		return dockerResourceSample{}, errors.New("docker stats memory or network value is invalid")
	}
	rss, err := parseDockerSize(memory[0])
	if err != nil {
		return dockerResourceSample{}, err
	}
	rx, err := parseDockerSize(network[0])
	if err != nil {
		return dockerResourceSample{}, err
	}
	tx, err := parseDockerSize(network[1])
	if err != nil {
		return dockerResourceSample{}, err
	}
	pids, err := strconv.Atoi(fields[5])
	if err != nil {
		return dockerResourceSample{}, err
	}
	return dockerResourceSample{containerID: fields[0], CPUCores: cpu / 100, RSSBytes: rss, RXBytes: rx, TXBytes: tx, PIDs: pids}, nil
}

func parseDockerSize(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	index := strings.IndexFunc(value, func(r rune) bool { return r != '.' && (r < '0' || r > '9') })
	if index <= 0 {
		return 0, errors.New("docker size has no unit")
	}
	number, err := strconv.ParseFloat(value[:index], 64)
	if err != nil || number < 0 {
		return 0, errors.New("docker size has an invalid number")
	}
	multipliers := map[string]float64{"B": 1, "kB": 1e3, "MB": 1e6, "GB": 1e9, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30}
	multiplier, found := multipliers[value[index:]]
	if !found {
		return 0, fmt.Errorf("docker size has unknown unit %q", value[index:])
	}
	return uint64(number * multiplier), nil
}

func marshalResourceSamples(samples []dockerResourceSample) ([]byte, error) {
	return json.MarshalIndent(samples, "", "  ")
}
