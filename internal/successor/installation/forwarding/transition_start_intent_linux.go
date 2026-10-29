package forwarding

import (
	"encoding/json"
	"path/filepath"
)

// Closed private versions only. A v1 cut has no invented intent; v2 is the
// actual Start producer's exact independently verifiable original preimage.
func transitionBoundarySchema(message transitionMessage) bool {
	if message.Schema == "ardents-forwarding-transition-boundary-v1" {
		return len(message.StartIntent) == 0
	}
	return message.Schema == "ardents-forwarding-transition-boundary-v2" && message.Operation == "start" && len(message.StartIntent) > 0 && len(message.StartIntent) <= 8<<10
}

type transitionStartIntent struct {
	initial, predecessor string
	number               attemptNumber
	history              uint8
}

// Canonical original intent, not authority or proof of its prior chain. Recovery
// must compare these commitments to independently retained actual history.
func readTransitionStartIntent(message transitionMessage) (transitionStartIntent, error) {
	var result transitionStartIntent
	if !transitionBoundarySchema(message) || message.Schema != "ardents-forwarding-transition-boundary-v2" {
		return result, ErrBinding
	}
	selected, err := decodeSelection(message.Binding)
	if err != nil || selected.binding.Generation != message.Generation || digest(selected.raw) != message.BindingSHA256 {
		return result, ErrBinding
	}
	var header struct {
		Initial     string `json:"initial_history_sha256"`
		Predecessor string `json:"predecessor_sha256"`
		Attempt     uint32 `json:"attempt"`
	}
	if json.Unmarshal(message.StartIntent, &header) != nil || header.Attempt >= attemptLimit || !validStartDigest(header.Initial) {
		return result, ErrBinding
	}
	result.initial, result.predecessor, result.number = header.Initial, header.Predecessor, attemptNumber(header.Attempt)
	if message.Journal != result.number.journal(selected.binding.Request.InstallationRoot) {
		return result, ErrBinding
	}
	history, extended, err := readStartHistoryIntent(message.StartIntent, selected.binding, message.BindingSHA256, result.initial, result.number, result.predecessor)
	if err != nil || !extended {
		return result, ErrBinding
	}
	result.history = history
	return result, nil
}

func checkTransitionStartIntent(message transitionMessage, records []initialRecord) error {
	if !transitionBoundarySchema(message) {
		return ErrBinding
	}
	if message.Schema == "ardents-forwarding-transition-boundary-v1" {
		return nil
	}
	intent, err := readTransitionStartIntent(message)
	if err != nil {
		return err
	}
	selected, err := decodeSelection(message.Binding)
	if err != nil {
		return err
	}
	guard := filepath.Join(intent.number.directory(selected.binding.Request.InstallationRoot), "start-guard.json")
	sha := digest(message.StartIntent)
	for _, row := range records {
		if row.Path == guard && (row.Phase == "file-intent" || row.Phase == "file-born" || row.Phase == "file-ready") || row.Phase == "start-attempt" || row.Phase == "invocation-observed" {
			if row.SHA256 != sha {
				return ErrBinding
			}
		}
	}
	return nil
}
