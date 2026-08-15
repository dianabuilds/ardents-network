package recoverysmoke

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func resourceHighWater(samples []recovery.ResourceSample) (uint64, float64) {
	var memory uint64
	var cpu float64
	for _, sample := range samples {
		memory = max(memory, sample.ClientRSS, sample.PublisherRSS)
		cpu = max(cpu, sample.ClientCPUPercent, sample.PublisherCPUPercent)
	}
	return memory, cpu
}

func (observer dockerObserver) streamResourceSamples(ctx context.Context, identities map[string]string,
	clock time.Time, ready func()) ([]recovery.ResourceSample, error) {
	services := []string{"client-endpoint", "client-app", "client", "publisher-endpoint", "publisher-app", "publisher"}
	args := []string{"stats", "--format", "{{json .}}"}
	for _, service := range services {
		args = append(args, identities[service])
	}
	command := exec.CommandContext(ctx, "docker", args...)
	stderr := &boundedTail{limit: 8 << 10}
	command.Dir, command.Stderr = observer.input.SourceRoot, stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	var samples []recovery.ResourceSample
	var sample recovery.ResourceSample
	seen := map[string]bool{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024), 64<<10)
	var parseErr error
	for scanner.Scan() {
		service, rowErr := addResourceRow(scanner.Bytes(), identities, services, &sample)
		if rowErr != nil {
			parseErr = rowErr
			break
		}
		if service == "" {
			continue
		}
		if seen[service] {
			sample, seen = recovery.ResourceSample{}, map[string]bool{}
			service, rowErr = addResourceRow(scanner.Bytes(), identities, services, &sample)
			if rowErr != nil {
				parseErr = rowErr
				break
			}
		}
		seen[service] = true
		if len(seen) == len(services) {
			sample.AtNanos = time.Since(clock).Nanoseconds()
			samples = appendResourceObservation(samples, sample)
			ready()
			sample, seen = recovery.ResourceSample{}, map[string]bool{}
		}
	}
	scanErr := scanner.Err()
	if parseErr != nil || scanErr != nil {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if ctx.Err() != nil && parseErr == nil && scanErr == nil {
		return samples, nil
	}
	if waitErr != nil {
		digest := sha256.Sum256(stderr.value())
		waitErr = fmt.Errorf("docker stats stream failed (stderr_sha256=%x): %w", digest, waitErr)
	}
	return samples, errors.Join(parseErr, scanErr, waitErr)
}

func appendResourceObservation(samples []recovery.ResourceSample,
	sample recovery.ResourceSample) []recovery.ResourceSample {
	if len(samples) == 0 {
		return append(samples, sample)
	}
	interval := sample.AtNanos - samples[len(samples)-1].AtNanos
	if interval >= int64(900*time.Millisecond) {
		return append(samples, sample)
	}
	if interval < 0 {
		return samples
	}
	if len(samples) == 1 {
		samples[0] = sample
		return samples
	}
	priorInterval := sample.AtNanos - samples[len(samples)-2].AtNanos
	if priorInterval >= int64(900*time.Millisecond) && priorInterval <= int64(1500*time.Millisecond) {
		samples[len(samples)-1] = sample
	}
	return samples
}

func addResourceRow(line []byte, identities map[string]string, services []string,
	result *recovery.ResourceSample) (string, error) {
	payload, present, err := dockerStatsPayload(line)
	if err != nil || !present {
		return "", err
	}
	var value struct{ ID, MemUsage, CPUPerc, NetIO string }
	if err := json.Unmarshal(payload, &value); err != nil {
		return "", fmt.Errorf("docker stats row is malformed: %w", err)
	}
	memory, received, sent, cpu, err := parseDockerStats(value.MemUsage, value.NetIO, value.CPUPerc)
	if err != nil {
		return "", err
	}
	matched := ""
	for _, service := range services {
		identity := identities[service]
		if validContainerID(identity) && (value.ID == identity || value.ID == identity[:12]) {
			if matched != "" {
				return "", errors.New("docker stats process identity is ambiguous")
			}
			matched = service
		}
	}
	for _, service := range services {
		if service == matched {
			if strings.HasPrefix(service, "client") {
				result.ClientRSS += memory
				result.ClientCPUPercent += cpu
			} else {
				result.PublisherRSS += memory
				result.PublisherCPUPercent += cpu
			}
			if service == "client" {
				result.ClientReceived, result.ClientSent = received, sent
			}
			if service == "publisher" {
				result.PublisherReceived, result.PublisherSent = received, sent
			}
			return service, nil
		}
	}
	return "", errors.New("docker stats process identity is unknown")
}

func dockerStatsPayload(line []byte) ([]byte, bool, error) {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	start, end := bytes.IndexByte(line, '{'), bytes.LastIndexByte(line, '}')
	if start < 0 && end < 0 {
		if dockerStatsControl(line) {
			return nil, false, nil
		}
		return nil, false, errors.New("docker stats row has no JSON payload")
	}
	if start < 0 || end < start || !dockerStatsControl(line[:start]) || !dockerStatsControl(line[end+1:]) {
		return nil, false, errors.New("docker stats row has invalid stream framing")
	}
	return line[start : end+1], true, nil
}

func dockerStatsControl(value []byte) bool {
	for len(value) > 0 {
		if value[0] == ' ' {
			value = value[1:]
			continue
		}
		if len(value) < 3 || value[0] != 0x1b || value[1] != '[' ||
			(value[2] != 'H' && value[2] != 'J' && value[2] != 'K') {
			return false
		}
		value = value[3:]
	}
	return true
}

func parseDockerStats(memoryText, networkText, cpuText string) (uint64, uint64, uint64, float64, error) {
	memory, err := dockerSize(strings.TrimSpace(strings.Split(memoryText, "/")[0]))
	parts := strings.Split(networkText, "/")
	if err != nil || len(parts) != 2 {
		return 0, 0, 0, 0, errors.New("docker memory or network stats are malformed")
	}
	received, receiveErr := dockerSize(strings.TrimSpace(parts[0]))
	sent, sendErr := dockerSize(strings.TrimSpace(parts[1]))
	cpu, cpuErr := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(cpuText), "%"), 64)
	if receiveErr != nil || sendErr != nil || cpuErr != nil || cpu < 0 {
		return 0, 0, 0, 0, errors.New("docker CPU or traffic stats are malformed")
	}
	return memory, received, sent, cpu, nil
}

func dockerSize(value string) (uint64, error) {
	units := []struct {
		suffix string
		factor float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, unit.suffix)), 64)
			if err != nil || number < 0 {
				return 0, errors.New("docker size is invalid")
			}
			return uint64(number * unit.factor), nil
		}
	}
	return 0, errors.New("docker size unit is unknown")
}
