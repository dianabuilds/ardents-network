package namedsite

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type measurementSummary struct {
	Attempts          int     `json:"attempts"`
	SetupP50Millis    int64   `json:"setup_p50_milliseconds"`
	SetupP95Millis    int64   `json:"setup_p95_milliseconds"`
	MaximumCPUCores   float64 `json:"maximum_cpu_cores"`
	MaximumRSSBytes   uint64  `json:"maximum_rss_bytes"`
	MaximumQueueBytes int     `json:"maximum_queue_bytes"`
	ApplicationBytes  uint64  `json:"application_bytes"`
}

func summarizeAttempts(evidenceDirectory string) (measurementSummary, error) {
	entries, err := os.ReadDir(filepath.Join(evidenceDirectory, "attempts"))
	if err != nil {
		return measurementSummary{}, err
	}
	var result measurementSummary
	var setup []int64
	for _, entry := range entries {
		if !entry.IsDir() {
			return measurementSummary{}, errors.New("attempt evidence has an unexpected file")
		}
		directory := filepath.Join(evidenceDirectory, "attempts", entry.Name())
		var run struct {
			SetupMilliseconds int64 `json:"setup_milliseconds"`
		}
		if err := readMeasurementJSON(filepath.Join(directory, "native-run.json"), &run); err != nil {
			return measurementSummary{}, err
		}
		setup = append(setup, run.SetupMilliseconds)
		var resources []struct {
			CPUCores float64 `json:"cpu_cores"`
			RSSBytes uint64  `json:"rss_bytes"`
		}
		if err := readMeasurementJSON(filepath.Join(directory, "resource-samples.json"), &resources); err != nil || len(resources) == 0 {
			return measurementSummary{}, errors.New("attempt resource observations are incomplete")
		}
		for _, sample := range resources {
			result.MaximumCPUCores = max(result.MaximumCPUCores, sample.CPUCores)
			result.MaximumRSSBytes = max(result.MaximumRSSBytes, sample.RSSBytes)
		}
		for _, role := range []string{"user", "service"} {
			var endpoint struct {
				ApplicationBytes    int `json:"application_bytes"`
				QueueHighWaterBytes int `json:"queue_high_water_bytes"`
			}
			if err := readMeasurementJSON(filepath.Join(directory, "native-roles", role+".json"), &endpoint); err != nil {
				return measurementSummary{}, err
			}
			result.ApplicationBytes += uint64(endpoint.ApplicationBytes)
			result.MaximumQueueBytes = max(result.MaximumQueueBytes, endpoint.QueueHighWaterBytes)
		}
	}
	if len(setup) == 0 {
		return measurementSummary{}, errors.New("no Gate C attempt measurements were retained")
	}
	sort.Slice(setup, func(i, j int) bool { return setup[i] < setup[j] })
	result.Attempts = len(setup)
	result.SetupP50Millis = setup[(len(setup)*50+99)/100-1]
	result.SetupP95Millis = setup[(len(setup)*95+99)/100-1]
	return result, nil
}

func readMeasurementJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 4*1024*1024 {
		return errors.New("bounded attempt observation is missing")
	}
	return json.Unmarshal(data, target)
}
