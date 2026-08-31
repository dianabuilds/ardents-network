//go:build referencec2

package main

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type carrierRelayReadyReceipt struct {
	Schema string `json:"schema"`
	Listen string `json:"listen"`
	Target string `json:"target"`
	PID    int    `json:"pid"`
}

func runCarrierRelay(input config) error {
	snapshot, err := serveCarrierRelay(input)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "carrier-relay",
		Class: "drained", Passed: true, CarrierRelay: &snapshot})
}

func serveCarrierRelay(input config) (carrierRelaySnapshot, error) {
	return serveCarrierRelayWithStart(input, startCarrierRelay)
}

func serveCarrierRelayWithStart(input config, start func(string, string) (*carrierRelay, error)) (carrierRelaySnapshot, error) {
	deadline, err := input.deadline()
	if err != nil {
		return carrierRelaySnapshot{}, err
	}
	relay, err := start(input.CarrierRelayListenAddress, input.CarrierRelayTargetAddress)
	if err != nil {
		return carrierRelaySnapshot{}, err
	}
	defer relay.close()
	ready := carrierRelayReadyReceipt{Schema: "ardents-h4-8-a11-carrier-relay-ready-v1", Listen: relay.endpoint(),
		Target: input.CarrierRelayTargetAddress, PID: os.Getpid()}
	if err := writeCarrierRelayJSON(input.CarrierRelayReadyPath, ready); err != nil {
		return carrierRelaySnapshot{}, err
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	reset := false
	for time.Now().Before(deadline) {
		select {
		case acceptErr := <-relay.acceptFailure:
			return carrierRelaySnapshot{}, acceptErr
		default:
		}
		if !reset {
			raw, readErr := os.ReadFile(input.CarrierRelayResetPath)
			if readErr == nil {
				if string(raw) != "reset\n" {
					return carrierRelaySnapshot{}, errors.New("carrier relay reset control is invalid")
				}
				receipt, resetErr := relay.reset()
				if resetErr != nil {
					return carrierRelaySnapshot{}, resetErr
				}
				if err := writeCarrierRelayJSON(input.CarrierRelayResetResultPath, receipt); err != nil {
					return carrierRelaySnapshot{}, err
				}
				reset = true
			} else if !errors.Is(readErr, os.ErrNotExist) {
				return carrierRelaySnapshot{}, readErr
			}
		}
		if info, statErr := os.Stat(input.CompletePath); statErr == nil && info.Mode().IsRegular() && info.Size() > 0 {
			select {
			case acceptErr := <-relay.acceptFailure:
				return carrierRelaySnapshot{}, acceptErr
			default:
			}
			relay.close()
			return relay.snapshot(), nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return carrierRelaySnapshot{}, statErr
		}
		select {
		case acceptErr := <-relay.acceptFailure:
			return carrierRelaySnapshot{}, acceptErr
		case <-ticker.C:
		}
	}
	return carrierRelaySnapshot{}, errors.New("carrier relay exceeded its fixture deadline")
}

func writeCarrierRelayJSON(path string, value any) error {
	if !filepath.IsAbs(path) {
		return errors.New("carrier relay receipt path is invalid")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func validateCarrierRelayConfig(input config) error {
	values := []string{input.CarrierRelayListenAddress, input.CarrierRelayTargetAddress, input.CarrierRelayReadyPath,
		input.CarrierRelayResetPath, input.CarrierRelayResetResultPath}
	configured := 0
	for _, value := range values {
		if value != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil
	}
	if configured != len(values) || !literalCarrierRelayEndpoint(input.CarrierRelayListenAddress, false) ||
		!literalCarrierRelayEndpoint(input.CarrierRelayTargetAddress, false) {
		return errors.New("C2 fixture Carrier relay configuration is incomplete")
	}
	listenHost, listenPort, _ := net.SplitHostPort(input.CarrierRelayListenAddress)
	targetHost, targetPort, _ := net.SplitHostPort(input.CarrierRelayTargetAddress)
	listenNumber, _ := strconv.Atoi(listenPort)
	targetNumber, _ := strconv.Atoi(targetPort)
	targetIP := net.ParseIP(targetHost)
	if net.ParseIP(listenHost).IsUnspecified() || targetIP == nil || !targetIP.IsLoopback() || listenNumber < 1024 || listenNumber != targetNumber {
		return errors.New("C2 fixture Carrier relay endpoints do not form the same-port loopback Adapter")
	}
	paths := []string{input.CarrierRelayReadyPath, input.CarrierRelayResetPath, input.CarrierRelayResetResultPath}
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return errors.New("C2 fixture Carrier relay path is invalid")
		}
		if _, exists := seen[path]; exists {
			return errors.New("C2 fixture Carrier relay paths are not distinct")
		}
		seen[path] = struct{}{}
	}
	return nil
}
