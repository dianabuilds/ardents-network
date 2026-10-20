//go:build linux

package main

import (
	"encoding/json"
	"errors"
)

// sourceServePlan declares only distribution. State owns current evidence and
// local serving guards; this command cannot acquire or refresh an Epoch.
type sourceServePlan struct {
	Network           *networkAuthorityPlan `json:"network"`
	ProcessProfile    string                `json:"process_profile"`
	ParticipationRoot string                `json:"participation_root"`
	Address           string                `json:"address"`
	Certificate       string                `json:"certificate"`
	PrivateKey        string                `json:"private_key"`
	ClientRoot        string                `json:"client_root"`
	ClientPins        [][32]byte            `json:"client_pins"`
}

// Go's fixed-array decoder accepts short and overlong arrays. This closed plan
// checks exact shapes before those decoded values can become trust declarations.
func decodeSourceServePlan(raw []byte) (sourceServePlan, error) {
	var plan sourceServePlan
	bad := errors.New("invalid Source serving plan")
	if decodeAdmissionObject(raw, &plan) != nil || !requiredAdmissionObject(raw, "network", "process_profile", "participation_root", "address", "certificate", "private_key", "client_root", "client_pins") {
		return plan, bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return plan, bad
	}
	if !requiredAdmissionObject(fields["network"], "root", "network_id", "authorities", "threshold", "profile_authority", "clock_observation_file") {
		return plan, bad
	}
	var network map[string]json.RawMessage
	if json.Unmarshal(fields["network"], &network) != nil {
		return plan, bad
	}
	exact := func(raw json.RawMessage) bool {
		var values []uint8
		return len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, &values) == nil && len(values) == 32
	}
	if !exact(network["network_id"]) || !exact(network["profile_authority"]) {
		return plan, bad
	}
	for _, raw := range []json.RawMessage{network["authorities"], fields["client_pins"]} {
		var pins []json.RawMessage
		if json.Unmarshal(raw, &pins) != nil || len(pins) == 0 {
			return plan, bad
		}
		for _, pin := range pins {
			if !exact(pin) {
				return plan, bad
			}
		}
	}
	return plan, nil
}
