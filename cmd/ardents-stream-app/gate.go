package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func progressGate(role string) (uint32, func(uint32) error, error) {
	text, root := os.Getenv("ARDENTS_STREAM_GATE_OFFSET"), os.Getenv("ARDENTS_STREAM_GATE_ROOT")
	if (text == "" || text == "0") && root == "" {
		return 0, func(uint32) error { return nil }, nil
	}
	parsed, err := strconv.ParseUint(text, 10, 32)
	if err != nil || parsed < 256<<10 || root != "/run/ardents/gate" {
		return 0, nil, errors.New("stream gate configuration is invalid")
	}
	ready, release := filepath.Join(root, role+".ready"), filepath.Join(root, role+".release")
	return uint32(parsed), func(received uint32) error {
		if received != uint32(parsed) {
			return errors.New("stream crossed its exact host-controlled gate")
		}
		return waitGateRelease(ready, release, received)
	}, nil
}

func progressGates(role string) ([]uint32, func(uint32) error, error) {
	text := os.Getenv("ARDENTS_STREAM_GATE_OFFSETS")
	if text == "" {
		offset, gate, err := progressGate(role)
		if offset == 0 {
			return nil, gate, err
		}
		return []uint32{offset}, gate, err
	}
	if os.Getenv("ARDENTS_STREAM_GATE_OFFSET") != "" || os.Getenv("ARDENTS_STREAM_GATE_ROOT") != "/run/ardents/gate" {
		return nil, nil, errors.New("sequential stream gate configuration is invalid")
	}
	parts := strings.Split(text, ",")
	if len(parts) == 0 || len(parts) > 3 {
		return nil, nil, errors.New("sequential stream gate count is outside its bound")
	}
	offsets := make([]uint32, len(parts))
	for index, part := range parts {
		parsed, err := strconv.ParseUint(part, 10, 32)
		if err != nil || parsed < 256<<10 || index > 0 && parsed <= uint64(offsets[index-1]) {
			return nil, nil, errors.New("sequential stream gate offset is invalid")
		}
		offsets[index] = uint32(parsed)
	}
	root := os.Getenv("ARDENTS_STREAM_GATE_ROOT")
	return offsets, func(received uint32) error {
		suffix := strconv.FormatUint(uint64(received), 10)
		return waitGateRelease(filepath.Join(root, role+".ready."+suffix),
			filepath.Join(root, role+".release."+suffix), received)
	}, nil
}

func waitGateRelease(ready, release string, received uint32) error {
	if err := publishGateReady(ready, received); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return errors.Join(err, errors.New("inspect host gate release "+release))
		}
		time.Sleep(time.Millisecond)
	}
	return errors.New("host did not release the exact stream gate")
}

func publishGateReady(path string, offset uint32) error {
	temporary := path + ".pending"
	if err := os.WriteFile(temporary, []byte(strconv.FormatUint(uint64(offset), 10)+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func gatedWorkloadWriter(write func([]byte) (int, error), offset uint32,
	gate func(uint32) error) func([]byte) (int, error) {
	return gatedWorkloadSequenceWriter(write, []uint32{offset}, gate)
}

func gatedWorkloadSequenceWriter(write func([]byte) (int, error), offsets []uint32,
	gate func(uint32) error) func([]byte) (int, error) {
	var written uint32
	next := 0
	return func(value []byte) (int, error) {
		count, err := write(value)
		written += uint32(count)
		if err == nil && next < len(offsets) && written == offsets[next] {
			err = gate(written)
			next++
		}
		if next < len(offsets) && written > offsets[next] {
			return count, errors.New("stream writer crossed its exact host-controlled gate")
		}
		return count, err
	}
}
