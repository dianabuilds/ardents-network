//go:build ignore

package main

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type trafficObservation struct {
	Interfaces int   `json:"interfaces"`
	RXBytes    int64 `json:"rx_bytes"`
	RXPackets  int64 `json:"rx_packets"`
	TXBytes    int64 `json:"tx_bytes"`
	TXPackets  int64 `json:"tx_packets"`
}

type resourceSample struct {
	OffsetMilliseconds int64              `json:"offset_milliseconds"`
	Harness            processObservation `json:"harness"`
	Client             processObservation `json:"client"`
	Server             processObservation `json:"server"`
	Traffic            trafficObservation `json:"traffic"`
	StateEntries       int                `json:"state_entries"`
	StateBytes         int64              `json:"state_bytes"`
}

func captureResourceSample(started time.Time, client, server *child, stateRoot string) (resourceSample, error) {
	sample := resourceSample{OffsetMilliseconds: time.Since(started).Milliseconds()}
	var err error
	if sample.Harness, err = observeProcess(os.Getpid()); err != nil {
		return sample, err
	}
	if sample.Client, err = observeProcess(client.cmd.Process.Pid); err != nil {
		return sample, err
	}
	if sample.Server, err = observeProcess(server.cmd.Process.Pid); err != nil {
		return sample, err
	}
	if sample.Traffic, err = observeTraffic(); err != nil {
		return sample, err
	}
	sample.StateEntries, sample.StateBytes, err = scanState(stateRoot)
	return sample, err
}

func observeTraffic() (trafficObservation, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return trafficObservation{}, err
	}
	defer file.Close()
	var result trafficObservation
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		_, values, _ := strings.Cut(line, ":")
		fields := strings.Fields(values)
		if len(fields) != 16 {
			return result, errors.New("unexpected /proc/net/dev row")
		}
		rxBytes, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return result, err
		}
		rxPackets, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return result, err
		}
		txBytes, err := strconv.ParseInt(fields[8], 10, 64)
		if err != nil {
			return result, err
		}
		txPackets, err := strconv.ParseInt(fields[9], 10, 64)
		if err != nil {
			return result, err
		}
		result.Interfaces++
		result.RXBytes += rxBytes
		result.RXPackets += rxPackets
		result.TXBytes += txBytes
		result.TXPackets += txPackets
	}
	return result, scanner.Err()
}
