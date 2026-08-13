package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
		if err := os.WriteFile(ready, []byte(strconv.FormatUint(uint64(received), 10)+"\n"), 0o600); err != nil {
			return err
		}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(release); err == nil {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
		return errors.New("host did not release the exact stream gate")
	}, nil
}

func gatedWorkloadWriter(write func([]byte) (int, error), offset uint32,
	gate func(uint32) error) func([]byte) (int, error) {
	var written uint32
	used := false
	return func(value []byte) (int, error) {
		count, err := write(value)
		written += uint32(count)
		if err == nil && !used && written == offset {
			used = true
			err = gate(written)
		}
		if !used && written > offset {
			return count, errors.New("stream writer crossed its exact host-controlled gate")
		}
		return count, err
	}
}
