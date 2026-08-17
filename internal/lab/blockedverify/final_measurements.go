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

func verifyFinalMeasurementContent(snapshots map[string][]byte, summary *finalSummary) []string {
	if summary == nil {
		return []string{"final measurement summary is missing"}
	}
	var reasons []string
	compareLines(snapshots, "measurements/profiles.jsonl", summary.Profiles, &reasons)
	compareLines(snapshots, "measurements/cells.jsonl", summary.Cells, &reasons)
	compareLines(snapshots, "measurements/capacity.jsonl", summary.Capacity, &reasons)
	compareLines(snapshots, "measurements/sustained.jsonl", summary.Sustained, &reasons)
	compareLines(snapshots, "measurements/pressure.jsonl", summary.Pressure, &reasons)
	recovery, err := decodeLines[finalRecovery](snapshots["measurements/recovery.jsonl"], 1)
	if err != nil || len(recovery) != 1 || !reflect.DeepEqual(recovery[0], summary.Recovery) {
		reasons = append(reasons, "final recovery measurements do not reproduce the summary")
	}
	wantResources, wantTraffic := summaryMeasurementRecords(summary)
	compareLines(snapshots, "measurements/resources.jsonl", wantResources, &reasons)
	compareLines(snapshots, "measurements/traffic.jsonl", wantTraffic, &reasons)
	var hosts finalHostRecord
	hostErr := decodeCanonicalSnapshot(snapshots["measurements/host.json"], &hosts)
	if hostErr != nil || hosts.Schema != "ardents-h3-final-host-v1" || !reflect.DeepEqual(hosts.Hosts, summary.Hosts) {
		reasons = append(reasons, "final host measurements do not reproduce the summary")
	}
	return reasons
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
