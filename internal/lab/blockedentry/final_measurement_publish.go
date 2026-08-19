package blockedentry

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

var finalMeasurementPaths = []string{
	"measurements/profiles.jsonl",
	"measurements/capacity.jsonl",
	"measurements/sustained.jsonl",
	"measurements/pressure.jsonl",
	"measurements/recovery.jsonl",
	"measurements/resources.jsonl",
	"measurements/traffic.jsonl",
	"measurements/host.json",
	"measurements/cells.jsonl",
}

type finalResourceRecord struct {
	Cell        string                   `json:"cell"`
	Observation finalResourceObservation `json:"observation"`
}

type finalTrafficRecord struct {
	Direction             string  `json:"direction"`
	EndpointCarrierRatio  float64 `json:"endpoint_carrier_ratio"`
	PublisherCarrierRatio float64 `json:"publisher_carrier_ratio"`
}

type finalHostRecord struct {
	Schema string              `json:"schema"`
	Hosts  []finalObservedHost `json:"hosts"`
}

func publishFinalMeasurements(root string, summary *finalSummary) error {
	if summary == nil {
		return nil
	}
	directory := filepath.Join(root, "measurements")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	resources, traffic := finalMeasurementRecords(summary)
	values := map[string]any{
		"profiles.jsonl":  summary.Profiles,
		"capacity.jsonl":  summary.Capacity,
		"sustained.jsonl": summary.Sustained,
		"pressure.jsonl":  summary.Pressure,
		"recovery.jsonl":  []finalRecovery{summary.Recovery},
		"resources.jsonl": resources,
		"traffic.jsonl":   traffic,
		"cells.jsonl":     summary.Cells,
	}
	for name, value := range values {
		if err := writeFinalJSONLines(filepath.Join(directory, name), value); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(directory, "host.json"),
		finalHostRecord{Schema: "ardents-h3-final-host-v1", Hosts: summary.Hosts}); err != nil {
		return err
	}
	summary.Artifacts = make([]artifactCommitment, 0, len(finalMeasurementPaths))
	for _, path := range finalMeasurementPaths {
		value, err := commitment(root, path)
		if err != nil {
			return err
		}
		summary.Artifacts = append(summary.Artifacts, value)
	}
	return nil
}

func writeFinalJSONLines(path string, values any) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) == 0 {
		return errors.Join(err, errors.New("final measurement sequence is empty or invalid"))
	}
	var output bytes.Buffer
	for _, entry := range entries {
		output.Write(entry)
		output.WriteByte('\n')
	}
	if output.Len() > maximumEvidenceFile {
		return errors.New("final measurement sequence is oversized")
	}
	return os.WriteFile(path, output.Bytes(), 0o600)
}

func finalMeasurementRecords(summary *finalSummary) ([]finalResourceRecord, []finalTrafficRecord) {
	resources := make([]finalResourceRecord, 0, len(summary.Capacity)+10)
	for _, value := range summary.Capacity {
		resources = append(resources, finalResourceRecord{Cell: capacityCellID(value), Observation: value.Resources})
	}
	traffic := make([]finalTrafficRecord, 0, len(summary.Sustained))
	for _, value := range summary.Sustained {
		for index, run := range value.Runs {
			resources = append(resources, finalResourceRecord{Cell: sustainedCellID(value.Direction, index),
				Observation: run.Resources})
		}
		traffic = append(traffic, finalTrafficRecord{Direction: value.Direction,
			EndpointCarrierRatio: value.EndpointCarrierRatio, PublisherCarrierRatio: value.PublisherCarrierRatio})
	}
	return resources, traffic
}

func capacityCellID(value finalCapacityBatch) string {
	return "capacity/" + value.Profile + "/" + strconv.Itoa(int(value.Batch))
}

func sustainedCellID(direction string, run int) string {
	return "sustained/" + direction + "/run-" + strconv.Itoa(run)
}
