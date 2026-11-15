package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// ServingDeclaration is the existing local Source command grammar. These are
// declarations only: State retains all signed authority/time/root decisions,
// Installation binds future inputs, and processresource admits actual placement.
type ServingDeclaration struct {
	Network           *ServingNetwork `json:"network"`
	ProcessProfile    string          `json:"process_profile"`
	ParticipationRoot string          `json:"participation_root"`
	Address           string          `json:"address"`
	Certificate       string          `json:"certificate"`
	PrivateKey        string          `json:"private_key"`
	ClientRoot        string          `json:"client_root"`
	ClientPins        [][32]byte      `json:"client_pins"`
}

// ServingNetwork is Source's embedded public State-input declaration, not an
// authenticated observation or a live root. Other command schemas stay separate.
type ServingNetwork struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

// DecodeServingDeclaration preserves exact field names, duplicate/null refusal,
// one complete object and exact 32-byte array shapes. It opens no resource.
// Both the runtime command and Installation consume this one byte grammar.
func DecodeServingDeclaration(raw []byte) (ServingDeclaration, error) {
	bad := errors.New("invalid Source serving declaration")
	var plan ServingDeclaration
	if len(raw) == 0 || len(raw) > 16<<10 {
		return plan, bad
	}
	fields, err := servingObject(raw, "network", "process_profile", "participation_root", "address", "certificate", "private_key", "client_root", "client_pins")
	if err != nil {
		return plan, bad
	}
	network, err := servingObject(fields["network"], "root", "network_id", "authorities", "threshold", "profile_authority", "clock_observation_file")
	if err != nil {
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
	if json.Unmarshal(raw, &plan) != nil {
		return ServingDeclaration{}, bad
	}
	return plan, nil
}

func servingObject(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	bad := errors.New("invalid Source serving object")
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return nil, bad
	}
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		value, err := d.Token()
		if err != nil {
			return nil, bad
		}
		key, ok := value.(string)
		if !ok || !wanted[key] {
			return nil, bad
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, bad
		}
		var body json.RawMessage
		if d.Decode(&body) != nil || bytes.Equal(body, []byte("null")) {
			return nil, bad
		}
		fields[key] = body
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || len(fields) != len(keys) {
		return nil, bad
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, bad
	}
	return fields, nil
}
