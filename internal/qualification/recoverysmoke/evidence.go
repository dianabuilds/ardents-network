package recoverysmoke

import (
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type applicationEvidence struct {
	Schema              string   `json:"schema"`
	Role                string   `json:"role"`
	Terminal            string   `json:"terminal"`
	SentBytes           uint32   `json:"sent_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	SentDigest          [32]byte `json:"sent_digest"`
	ReceivedDigest      [32]byte `json:"received_digest"`
	ResultClass         string   `json:"result_class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	SendSeed            [32]byte `json:"send_seed"`
	ExpectSeed          [32]byte `json:"expect_seed"`
}

func terminalEndpoint(raw []byte) (serviceconn.Result, error) {
	var terminal serviceconn.Result
	count := 0
	for _, line := range splitLines(raw) {
		var value serviceconn.Result
		if json.Unmarshal(line, &value) == nil && value.Class != "" {
			terminal = value
			count++
		}
	}
	if count != 1 {
		return serviceconn.Result{}, errors.New("endpoint terminal evidence count is not exactly one")
	}
	return terminal, nil
}

func terminalApplication(raw []byte) (applicationEvidence, error) {
	var terminal applicationEvidence
	for _, line := range splitLines(raw) {
		if json.Unmarshal(line, &terminal) == nil && terminal.Terminal != "" {
			break
		}
	}
	if terminal.Terminal == "" {
		return terminal, errors.New("application terminal evidence is missing")
	}
	return terminal, nil
}

func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for index, value := range raw {
		if value == '\n' {
			if index > start {
				lines = append(lines, raw[start:index])
			}
			start = index + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
