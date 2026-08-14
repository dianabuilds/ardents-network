package recoverysmoke

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

func readNetworkTrafficFile() (trafficCounterReceipt, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return trafficCounterReceipt{}, err
	}
	value, readErr := readNetworkTraffic(file)
	return value, errors.Join(readErr, file.Close())
}

func readNetworkTraffic(input io.Reader) (trafficCounterReceipt, error) {
	result := trafficCounterReceipt{Kind: "traffic"}
	raw, err := io.ReadAll(io.LimitReader(input, (64<<10)+1))
	if err != nil {
		return trafficCounterReceipt{}, err
	}
	if len(raw) > 64<<10 {
		return trafficCounterReceipt{}, errors.New("network traffic table exceeds its byte bound")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), 4<<10)
	lines := 0
	for scanner.Scan() {
		lines++
		if lines > 64 {
			return trafficCounterReceipt{}, errors.New("network traffic table exceeds its interface bound")
		}
		name, values, found := strings.Cut(scanner.Text(), ":")
		name = strings.TrimSpace(name)
		if !found || name == "lo" {
			continue
		}
		if name == "" {
			return trafficCounterReceipt{}, errors.New("network traffic interface name is empty")
		}
		fields := strings.Fields(values)
		if len(fields) != 16 {
			return trafficCounterReceipt{}, errors.New("network traffic interface row is malformed")
		}
		received, receiveErr := strconv.ParseUint(fields[0], 10, 64)
		sent, sendErr := strconv.ParseUint(fields[8], 10, 64)
		if receiveErr != nil || sendErr != nil || ^uint64(0)-result.Received < received || ^uint64(0)-result.Sent < sent {
			return trafficCounterReceipt{}, errors.New("network traffic counter is invalid")
		}
		result.Interfaces++
		result.Received += received
		result.Sent += sent
	}
	if err := scanner.Err(); err != nil {
		return trafficCounterReceipt{}, err
	}
	if result.Interfaces == 0 {
		return trafficCounterReceipt{}, errors.New("network traffic table has no observed interface")
	}
	return result, nil
}
