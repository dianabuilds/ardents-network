package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

func runDirectCommand(arguments []string, output io.Writer) error {
	if len(arguments) != 4 {
		return errors.New("direct workload requires an address, seed file, and byte count")
	}
	seed, err := readSeed(arguments[2])
	if err != nil {
		return err
	}
	count, _, err := streamCounts(arguments[3], "0")
	if err != nil {
		return err
	}
	lifetime, err := streamLifetime()
	if err != nil {
		return err
	}
	delay, err := directStartDelay()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), lifetime)
	defer cancel()
	if err := waitDirectStart(ctx, os.Getenv("ARDENTS_STREAM_START_FILE")); err != nil {
		return err
	}
	config := DirectConfig{Role: arguments[0], Address: arguments[1], Seed: seed,
		Bytes: count, Output: output, StartDelay: delay}
	if encoded := os.Getenv("ARDENTS_STREAM_MEASURE_DURATION"); encoded != "" {
		config.MeasureDuration, err = time.ParseDuration(encoded)
		if err != nil || config.MeasureDuration <= 0 {
			return errors.New("direct workload measurement duration is invalid")
		}
	}
	if arguments[0] == "direct-listen" {
		encoder := json.NewEncoder(output)
		config.Ready = func(address string) {
			_ = encoder.Encode(map[string]string{"kind": "ready", "address": address})
		}
	}
	return Direct(ctx, config)
}

func waitDirectStart(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func directStartDelay() (time.Duration, error) {
	value := os.Getenv("ARDENTS_STREAM_START_DELAY")
	if value == "" {
		return 0, nil
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 || delay > 5*time.Second {
		return 0, errors.New("direct workload start delay is outside its bound")
	}
	return delay, nil
}
