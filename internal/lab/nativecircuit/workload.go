package nativecircuit

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

const (
	nativeWorkloadSchema = "carrier-lab-native-workload/v1"
	c3Profile            = "carrier-lab-c3/v1"
	workloadC5           = "c5-c2"
	workloadC3           = "c3"
	workloadDirect       = "direct"
	directProfile        = "carrier-lab-direct/v1"
)

type nativeWorkload struct {
	SchemaVersion   string `json:"schema_version"`
	Profile         string `json:"profile"`
	Kind            string `json:"kind"`
	Direction       string `json:"direction,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	Seed            string `json:"seed"`
}

func readNativeWorkload(path string) (nativeWorkload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nativeWorkload{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var workload nativeWorkload
	if err := decoder.Decode(&workload); err != nil {
		return nativeWorkload{}, errors.New("native qualification workload has invalid encoding")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nativeWorkload{}, errors.New("native qualification workload has trailing data")
	}
	if err := validateNativeWorkload(workload); err != nil {
		return nativeWorkload{}, err
	}
	return workload, nil
}

func validateNativeWorkload(workload nativeWorkload) error {
	seed, err := hex.DecodeString(workload.Seed)
	if err != nil || len(seed) != 32 || hex.EncodeToString(seed) != workload.Seed {
		return errors.New("native qualification workload seed must be 32 canonical bytes")
	}
	if workload.SchemaVersion != nativeWorkloadSchema || workload.Profile != workloadC5 && workload.Profile != workloadC3 && workload.Profile != workloadDirect {
		return errors.New("native qualification workload schema or profile is invalid")
	}
	if workload.Kind == "setup" && workload.Direction == "" && workload.DurationSeconds == 0 {
		return nil
	}
	if workload.Kind != "stream" {
		return errors.New("native qualification workload kind is invalid")
	}
	spec := streamSpec{Direction: workload.Direction, Seed: workload.Seed, Duration: time.Duration(workload.DurationSeconds) * time.Second}
	return validateStreamSpec(spec, true)
}
