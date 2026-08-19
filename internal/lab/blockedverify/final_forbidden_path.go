package blockedverify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

type finalForbiddenPathReceipt struct {
	Schema                  string          `json:"schema"`
	Variant                 string          `json:"variant"`
	Source                  string          `json:"source"`
	Component               string          `json:"component"`
	InputSHA256             string          `json:"input_sha256"`
	Calls                   uint16          `json:"calls"`
	ContactStarts           uint16          `json:"contact_starts"`
	Terminal                string          `json:"terminal"`
	DeadlineOffset          uint64          `json:"deadline_offset"`
	CandidateContract       json.RawMessage `json:"candidate_contract"`
	CandidateContractSHA256 string          `json:"candidate_contract_sha256"`
}

type finalG7ComponentContract struct {
	Schema           string          `json:"schema"`
	Variant          string          `json:"variant"`
	Component        string          `json:"component"`
	Input            json.RawMessage `json:"input"`
	ReachableTargets []string        `json:"reachable_targets"`
	ObservedTargets  []string        `json:"observed_targets"`
	ChildEnvironment []string        `json:"child_environment"`
	StateEntries     []string        `json:"state_entries"`
	EntryError       string          `json:"entry_error"`
}

type finalUnknownInviteReceipt struct {
	Schema           string `json:"schema"`
	BaselineTerminal string `json:"baseline_terminal"`
	Terminal         string `json:"terminal"`
	BeforeInput      []byte `json:"before_input"`
	AfterInput       []byte `json:"after_input"`
}

func validFinalFaultReceipt(value *finalFaultExercise, group, variant string) bool {
	if len(value.Receipt) == 0 || len(value.Receipt) > 16<<10 {
		return false
	}
	digest := sha256.Sum256(value.Receipt)
	if value.ReceiptSHA256 != hex.EncodeToString(digest[:]) {
		return false
	}
	switch group {
	case "G4-restart":
		return validFinalG4Receipt(value.Receipt, variant)
	case "G7-forbidden-path":
		return validFinalForbiddenPathReceipt(value.Receipt, variant)
	case "G9-ledger-leakage":
		return variant != "unknown-invite-field" || validFinalUnknownInviteReceipt(value.Receipt)
	default:
		return true
	}
}

func validFinalUnknownInviteReceipt(raw []byte) bool {
	var value finalUnknownInviteReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	canonical, err := json.Marshal(value)
	return err == nil && bytes.Equal(canonical, raw) && value.Schema == "ardents-h3-g9-unknown-invite-v1" &&
		value.BaselineTerminal == "accepted" && value.Terminal == "invalid" &&
		len(value.BeforeInput) > 0 && len(value.BeforeInput) <= 8<<10 &&
		len(value.AfterInput) == len(value.BeforeInput)+1 && value.AfterInput[len(value.AfterInput)-1] == 0 &&
		bytes.Equal(value.AfterInput[:len(value.BeforeInput)], value.BeforeInput)
}

func validFinalForbiddenPathReceipt(raw []byte, variant string) bool {
	var value finalForbiddenPathReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	sources := map[string]string{"dns": "host-alias", "environment-proxy": "proxy-environment",
		"ordinary-entry": "ordinary-entry-address", "direct-target": "direct-target-address",
		"alternate-address": "uncommitted-address", "alternate-candidate": "uncommitted-candidate",
		"shorter-route": "short-route-address", "cached-success": "prior-success-cache",
		"deadline-exposure-reset": "reset-request"}
	components := map[string]string{"dns": "adapter-resolver", "environment-proxy": "adapter-process",
		"ordinary-entry": "route-entry", "direct-target": "endpoint-route", "alternate-address": "adapter-config",
		"alternate-candidate": "bridge-ledger", "shorter-route": "route-plan", "cached-success": "adapter-state",
		"deadline-exposure-reset": "bridge-attempt"}
	terminal := "bridge-attempt-exhausted"
	if variant == "deadline-exposure-reset" {
		terminal = "bridge-deadline-exceeded"
	}
	return value.Schema == "ardents-h3-g7-receipt-v2" &&
		value.Variant == variant && value.Source == sources[variant] && value.Component == components[variant] &&
		validFinalG7Component(value, variant, components[variant]) && value.Calls > 0 &&
		value.Calls <= 2 && value.ContactStarts == value.Calls && value.Terminal == terminal &&
		value.DeadlineOffset > 0
}

func validFinalG7Component(value finalForbiddenPathReceipt, variant, component string) bool {
	digest := sha256.Sum256(value.CandidateContract)
	if value.CandidateContractSHA256 != hex.EncodeToString(digest[:]) {
		return false
	}
	var contract finalG7ComponentContract
	decoder := json.NewDecoder(bytes.NewReader(value.CandidateContract))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&contract) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		contract.Schema != "ardents-h3-g7-component-v1" || contract.Variant != variant ||
		contract.Component != component || len(contract.Input) == 0 || len(contract.ReachableTargets) == 0 {
		return false
	}
	canonical, err := json.Marshal(contract)
	if err != nil || !bytes.Equal(canonical, value.CandidateContract) {
		return false
	}
	inputDigest := sha256.Sum256(contract.Input)
	if value.InputSHA256 != hex.EncodeToString(inputDigest[:]) {
		return false
	}
	if component == "route-entry" || component == "endpoint-route" || component == "route-plan" {
		return validFinalG7RouteContract(contract, variant)
	}
	return validFinalG7AdapterContract(contract)
}

func validFinalG7RouteContract(contract finalG7ComponentContract, variant string) bool {
	var input struct {
		Variant string `json:"variant"`
		Plan    struct {
			Positions []struct {
				Endpoint string `json:"Endpoint"`
			} `json:"Positions"`
		} `json:"plan"`
		DirectAddress string `json:"direct_address"`
	}
	if json.Unmarshal(contract.Input, &input) != nil || input.Variant != variant || len(input.Plan.Positions) != 4 ||
		len(contract.ReachableTargets) != 5 || len(contract.ObservedTargets) != 0 ||
		len(contract.ChildEnvironment) != 0 || len(contract.StateEntries) != 0 ||
		contract.EntryError != "bridge-attempt-exhausted" || input.DirectAddress != contract.ReachableTargets[4] {
		return false
	}
	for index, position := range input.Plan.Positions {
		if position.Endpoint != contract.ReachableTargets[index] {
			return false
		}
	}
	return true
}

func validFinalG7AdapterContract(contract finalG7ComponentContract) bool {
	var input struct {
		CandidateEnvelope string   `json:"candidate_envelope"`
		AmbientProxy      []string `json:"ambient_proxy"`
	}
	if json.Unmarshal(contract.Input, &input) != nil || len(input.AmbientProxy) != 3 ||
		len(contract.ReachableTargets) != 1 || len(contract.ObservedTargets) != 1 ||
		contract.ReachableTargets[0] != contract.ObservedTargets[0] || len(contract.StateEntries) != 0 ||
		contract.EntryError != "" || !validFinalG7Environment(contract.ChildEnvironment) {
		return false
	}
	for _, proxy := range input.AmbientProxy {
		if proxy == "" {
			return false
		}
	}
	raw, err := hex.DecodeString(input.CandidateEnvelope)
	return err == nil && finalG7EnvelopeTarget(raw) == contract.ObservedTargets[0]
}

func validFinalG7Environment(values []string) bool {
	if len(values) != 4 || values[0] != "TOR_PT_CLIENT_TRANSPORTS=webtunnel" ||
		values[1] != "TOR_PT_EXIT_ON_STDIN_CLOSE=1" || values[2] != "TOR_PT_MANAGED_TRANSPORT_VER=1" {
		return false
	}
	return len(values[3]) > len("TOR_PT_STATE_LOCATION=/") &&
		bytes.HasPrefix([]byte(values[3]), []byte("TOR_PT_STATE_LOCATION=/"))
}

func finalG7EnvelopeTarget(raw []byte) string {
	const magic = "ardents-h3-wt1"
	if len(raw) < len(magic)+2 || string(raw[:len(magic)]) != magic || raw[len(magic)] != 1 {
		return ""
	}
	profileLength := int(raw[len(magic)+1])
	offset := len(magic) + 2 + profileLength
	if profileLength != len("webtunnel-v0.0.6") || len(raw) < offset+6 ||
		string(raw[len(magic)+2:offset]) != "webtunnel-v0.0.6" {
		return ""
	}
	port := uint16(raw[offset+4])<<8 | uint16(raw[offset+5])
	address := net.IP(raw[offset : offset+4]).String()
	offset += 6
	if len(raw) < offset+2 {
		return ""
	}
	pathLength := int(raw[offset])<<8 | int(raw[offset+1])
	offset += 2
	if pathLength != len("/entry") || len(raw) < offset+pathLength+1 || string(raw[offset:offset+pathLength]) != "/entry" {
		return ""
	}
	offset += pathLength
	nameLength := int(raw[offset])
	offset++
	if nameLength != len("front.example") || len(raw) != offset+nameLength+32 ||
		string(raw[offset:offset+nameLength]) != "front.example" || bytes.Equal(raw[len(raw)-32:], make([]byte, 32)) {
		return ""
	}
	return net.JoinHostPort(address, fmt.Sprint(port))
}
