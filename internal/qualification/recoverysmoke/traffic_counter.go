package recoverysmoke

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var errNoNetworkTrafficInterface = errors.New("network traffic table has no observed interface")

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
		return trafficCounterReceipt{}, errNoNetworkTrafficInterface
	}
	return result, nil
}

func retainNetworkTraffic(ctx context.Context, reports <-chan os.Signal, ready func(),
	emit func(trafficCounterReceipt) error, read func() (trafficCounterReceipt, error)) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var retained trafficCounterReceipt
	retained.Kind = "traffic"
	if ready != nil {
		ready()
	}
	for {
		value, err := read()
		if err == nil {
			retained.Interfaces = max(retained.Interfaces, value.Interfaces)
			retained.Received = max(retained.Received, value.Received)
			retained.Sent = max(retained.Sent, value.Sent)
		} else if !errors.Is(err, errNoNetworkTrafficInterface) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-reports:
			if retained.Interfaces == 0 || retained.Received == 0 || retained.Sent == 0 {
				return errors.New("network traffic receipt was requested before a complete sample")
			}
			if err := emit(retained); err != nil {
				return err
			}
		}
	}
}
