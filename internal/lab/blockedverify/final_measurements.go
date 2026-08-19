package blockedverify

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

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

func recomputeFinalSummary(snapshots map[string][]byte, published *finalSummary) (*finalSummary, []string) {
	if published == nil {
		return nil, []string{"final measurement summary is missing"}
	}
	result := &finalSummary{Schema: published.Schema, ImageHash: published.ImageHash,
		ClientHash: published.ClientHash, ServerHash: published.ServerHash, Artifacts: published.Artifacts,
		MutationArtifacts: published.MutationArtifacts}
	var reasons []string
	result.Profiles = decodeFinalLines[finalProfileResult](snapshots, "measurements/profiles.jsonl", 7, &reasons)
	result.Cells = decodeFinalLines[finalCellObservation](snapshots, "measurements/cells.jsonl", 564, &reasons)
	result.Capacity = decodeFinalLines[finalCapacityBatch](snapshots, "measurements/capacity.jsonl", 10, &reasons)
	result.Sustained = decodeFinalLines[finalSustainedCell](snapshots, "measurements/sustained.jsonl", 2, &reasons)
	result.Pressure = decodeFinalLines[finalPressureCell](snapshots, "measurements/pressure.jsonl", 5, &reasons)
	recovery := decodeFinalLines[finalRecovery](snapshots, "measurements/recovery.jsonl", 1, &reasons)
	if len(recovery) == 1 {
		result.Recovery = recovery[0]
	}
	var hosts finalHostRecord
	if err := decodeCanonicalSnapshot(snapshots["measurements/host.json"], &hosts); err != nil ||
		hosts.Schema != "ardents-h3-final-host-v1" {
		reasons = append(reasons, "final host measurements are invalid")
	} else {
		result.Hosts = hosts.Hosts
	}
	wantResources, wantTraffic := summaryMeasurementRecords(result)
	compareLines(snapshots, "measurements/resources.jsonl", wantResources, &reasons)
	compareLines(snapshots, "measurements/traffic.jsonl", wantTraffic, &reasons)
	if !reflect.DeepEqual(*result, *published) {
		reasons = append(reasons, "final published summary differs from raw measurements")
	}
	return result, reasons
}

func decodeFinalLines[T any](snapshots map[string][]byte, relative string, count int, reasons *[]string) []T {
	values, err := decodeLines[T](snapshots[relative], count)
	if err != nil {
		*reasons = append(*reasons, "final raw measurements do not reproduce "+relative)
		return nil
	}
	return values
}

func compareLines[T any](snapshots map[string][]byte, relative string, wanted []T, reasons *[]string) {
	values, err := decodeLines[T](snapshots[relative], len(wanted))
	if err != nil || !reflect.DeepEqual(values, wanted) {
		*reasons = append(*reasons, "final raw measurements do not reproduce "+relative)
	}
}

func decodeLines[T any](raw []byte, maximum int) ([]T, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), maximumInput)
	result := make([]T, 0, maximum)
	for scanner.Scan() {
		if len(result) >= maximum {
			return nil, errors.New("measurement JSONL has trailing records")
		}
		line := append([]byte(nil), scanner.Bytes()...)
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		var value T
		if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return nil, errors.New("measurement JSONL record is not strict")
		}
		canonical, err := json.Marshal(value)
		if err != nil || !bytes.Equal(canonical, line) {
			return nil, errors.New("measurement JSONL record is not canonical")
		}
		result = append(result, value)
	}
	if err := scanner.Err(); err != nil || len(result) != maximum {
		return nil, errors.Join(err, fmt.Errorf("measurement JSONL count=%d want=%d", len(result), maximum))
	}
	return result, nil
}

func decodeCanonicalSnapshot(raw []byte, value any) error {
	if len(raw) == 0 {
		return errors.New("measurement JSON snapshot is missing")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("measurement JSON snapshot is not strict")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("measurement JSON snapshot is not canonical")
	}
	return nil
}

func summaryMeasurementRecords(summary *finalSummary) ([]finalResourceRecord, []finalTrafficRecord) {
	resources := make([]finalResourceRecord, 0, len(summary.Capacity)+10)
	for _, value := range summary.Capacity {
		resources = append(resources, finalResourceRecord{Cell: fmt.Sprintf("capacity/%s/%d", value.Profile, value.Batch),
			Observation: value.Resources})
	}
	traffic := make([]finalTrafficRecord, 0, len(summary.Sustained))
	for _, cell := range summary.Sustained {
		for index, run := range cell.Runs {
			resources = append(resources, finalResourceRecord{Cell: fmt.Sprintf("sustained/%s/run-%d", cell.Direction, index),
				Observation: run.Resources})
		}
		traffic = append(traffic, finalTrafficRecord{Direction: cell.Direction,
			EndpointCarrierRatio: cell.EndpointCarrierRatio, PublisherCarrierRatio: cell.PublisherCarrierRatio})
	}
	return resources, traffic
}
