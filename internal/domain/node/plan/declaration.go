package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

// This file owns the original node-role runtime invocation declaration grammar:
// a pure closed byte codec mirroring network/source/servingdeclaration.ServingDeclaration. It is
// configuration validation kept with the runtime owner (ADR-0125 decision 6).
// It performs no State acceptance, no root creation, no hosting initialization
// and no processguard policy; those stay with their separate owners. The
// command composition converts these declarations into state.Config. Runtime invocation
// validation checks the fixed role profile and separated roots. The codec returns
// facts only and opens no resource.

// Receiving is the closed set of receiving role declarations.
type Receiving interface {
	ForwardingPlan | DescriptorPlan | IntroductionPlan | JoinPlan
	ValidInvocation() bool
	InvocationTarget() (*Network, string, [32]byte, uint32)
}

// Network is the embedded public Network-input declaration shared by every
// node role. It is layout-identical to the command composition's local Network
// owner plan so composition can adapt it by pointer conversion. It is not an
// authenticated observation or a live root; signed Epoch/profile bytes and their
// durable history supply the actual Network facts.
type Network struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

// nodePlanCore is the common declaration core shared by all five node roles.
// Each role embeds it and adds only its own extension fields, so the closed
// decoder can enforce one exact allowed field set per role. Omitted fields
// decode to their zero value, preserving the existing runtime plan grammar;
// the per-role command policy rejects incomplete declarations before effect.
type nodePlanCore struct {
	Network           *Network        `json:"network"`
	NodeID            [32]byte        `json:"node_id"`
	ProcessProfile    string          `json:"process_profile"`
	ParticipationRoot string          `json:"participation_root"`
	SpendRoot         string          `json:"spend_root"`
	HostingRoot       string          `json:"hosting_root"`
	Certificate       string          `json:"certificate"`
	PrivateKey        string          `json:"private_key"`
	Work              hosting.Traffic `json:"work"`
	Termination       hosting.Traffic `json:"termination"`
	RetirementWaitMS  uint32          `json:"retirement_wait_ms"`
}

// InvocationTarget returns the declaration fields used to open State and join
// the original runtime. The selected role validates the declaration separately.
func (plan nodePlanCore) InvocationTarget() (*Network, string, [32]byte, uint32) {
	return plan.Network, plan.ParticipationRoot, plan.NodeID, plan.RetirementWaitMS
}

// ForwardingPlan is the original Forwarding invocation declaration: the common
// core only, with no role extension field.
type ForwardingPlan struct {
	nodePlanCore
}

// JoinPlan is the original Rendezvous JOIN invocation declaration: the common
// core only, with no role extension field.
type JoinPlan struct {
	nodePlanCore
}

// DescriptorPlan adds the separately owned Reachability Store root.
type DescriptorPlan struct {
	nodePlanCore
	DescriptorRoot string `json:"descriptor_root"`
}

// IntroductionPlan adds the prepared Route Registry/History root.
type IntroductionPlan struct {
	nodePlanCore
	IntroductionRoot string `json:"introduction_root"`
}

// IssuerPlan adds the raw initialize-results sub-plan (whose own grammar stays
// with the command composition) and the bootstrap Hosting reservation.
type IssuerPlan struct {
	nodePlanCore
	Issuance      json.RawMessage `json:"issuance"`
	BootstrapWork hosting.Traffic `json:"bootstrap_work"`
}

// DecodeNodeNetwork preserves exact field names, duplicate/null refusal, one
// complete object and exact 32-byte array shapes for the embedded Network
// declaration. It opens no resource and grants no Network authority.
func DecodeNodeNetwork(raw []byte) (Network, error) {
	bad := errors.New("invalid node network declaration")
	var network Network
	if len(raw) == 0 || len(raw) > 16<<10 {
		return network, bad
	}
	fields, err := nodeObject(raw, "root", "network_id", "authorities", "threshold", "profile_authority", "clock_observation_file")
	if err != nil || !nodeNetworkShapes(fields) {
		return network, bad
	}
	if json.Unmarshal(raw, &network) != nil {
		return Network{}, bad
	}
	return network, nil
}

// ValidInvocation checks the fixed role profile, retirement bound and distinct roots.
func (plan ForwardingPlan) ValidInvocation() bool {
	return plan.nodePlanCore.validInvocation(processguard.ForwardingProfile)
}

func (plan JoinPlan) ValidInvocation() bool {
	return plan.nodePlanCore.validInvocation(processguard.JoinProfile)
}

func (plan DescriptorPlan) ValidInvocation() bool {
	return plan.nodePlanCore.validInvocation(processguard.DescriptorProfile, plan.DescriptorRoot)
}

func (plan IntroductionPlan) ValidInvocation() bool {
	return plan.nodePlanCore.validInvocation(processguard.IntroductionProfile, plan.IntroductionRoot)
}

func (plan IssuerPlan) ValidInvocation(admissionRoot, keyRoot, resultRoot string) bool {
	return plan.nodePlanCore.validInvocation(processguard.IssuerProfile, admissionRoot, keyRoot, resultRoot)
}

func (plan nodePlanCore) validInvocation(profile string, roleRoots ...string) bool {
	return plan.Network != nil && plan.ProcessProfile == profile &&
		plan.RetirementWaitMS > 0 && plan.RetirementWaitMS <= 5000 &&
		plan.Network.IndependentRoots(append([]string{plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot}, roleRoots...)...)
}

// DecodeForwardingPlan decodes the exact Forwarding declaration field set.
func DecodeForwardingPlan(raw []byte) (ForwardingPlan, error) {
	bad := errors.New("invalid Forwarding node plan")
	var plan ForwardingPlan
	if _, err := nodePlanFields(raw); err != nil {
		return plan, bad
	}
	if json.Unmarshal(raw, &plan) != nil {
		return ForwardingPlan{}, bad
	}
	return plan, nil
}

// DecodeJoinPlan decodes the exact JOIN declaration field set.
func DecodeJoinPlan(raw []byte) (JoinPlan, error) {
	bad := errors.New("invalid JOIN node plan")
	var plan JoinPlan
	if _, err := nodePlanFields(raw); err != nil {
		return plan, bad
	}
	if json.Unmarshal(raw, &plan) != nil {
		return JoinPlan{}, bad
	}
	return plan, nil
}

// DecodeDescriptorPlan decodes the exact Descriptor declaration field set,
// including its Store root and no other role's extension.
func DecodeDescriptorPlan(raw []byte) (DescriptorPlan, error) {
	bad := errors.New("invalid Descriptor node plan")
	var plan DescriptorPlan
	if _, err := nodePlanFields(raw, "descriptor_root"); err != nil {
		return plan, bad
	}
	if json.Unmarshal(raw, &plan) != nil {
		return DescriptorPlan{}, bad
	}
	return plan, nil
}

// DecodeIntroductionPlan decodes the exact Introduction declaration field set.
// The retained repair command consumes this same full grammar as the runtime.
func DecodeIntroductionPlan(raw []byte) (IntroductionPlan, error) {
	bad := errors.New("invalid Introduction node plan")
	var plan IntroductionPlan
	if _, err := nodePlanFields(raw, "introduction_root"); err != nil {
		return plan, bad
	}
	if json.Unmarshal(raw, &plan) != nil {
		return IntroductionPlan{}, bad
	}
	return plan, nil
}

// DecodeIssuerPlan decodes the exact Issuer declaration field set. The issuance
// sub-plan is retained as raw bytes; its initialize-results grammar and the
// per-role root policy stay with the command composition.
func DecodeIssuerPlan(raw []byte) (IssuerPlan, error) {
	bad := errors.New("invalid Issuer node plan")
	var plan IssuerPlan
	fields, err := nodePlanFields(raw, "issuance", "bootstrap_work")
	if err != nil {
		return plan, bad
	}
	if body, ok := fields["bootstrap_work"]; ok {
		if _, err := nodeObject(body, "tx", "rx"); err != nil {
			return plan, bad
		}
	}
	if json.Unmarshal(raw, &plan) != nil {
		return IssuerPlan{}, bad
	}
	return plan, nil
}

// nodePlanFields validates the common core grammar plus the role's extension
// keys. It enforces the exact allowed field set (no foreign role field),
// duplicate/null refusal and exact array/Traffic shapes for present fields.
func nodePlanFields(raw []byte, extension ...string) (map[string]json.RawMessage, error) {
	bad := errors.New("invalid node plan declaration")
	if len(raw) == 0 || len(raw) > 16<<10 {
		return nil, bad
	}
	keys := []string{"network", "node_id", "process_profile", "participation_root", "spend_root", "hosting_root", "certificate", "private_key", "work", "termination", "retirement_wait_ms"}
	keys = append(keys, extension...)
	fields, err := nodeObject(raw, keys...)
	if err != nil {
		return nil, bad
	}
	if _, err := DecodeNodeNetwork(fields["network"]); err != nil {
		return nil, bad
	}
	if body, ok := fields["node_id"]; ok && !exactNodeBytes(body) {
		return nil, bad
	}
	for _, name := range []string{"work", "termination"} {
		if body, ok := fields[name]; ok {
			if _, err := nodeObject(body, "tx", "rx"); err != nil {
				return nil, bad
			}
		}
	}
	return fields, nil
}

// nodeNetworkShapes checks the exact 32-byte identities and authority pin shapes
// for the fields that are present. Omitted identities decode to zero and are
// refused by the command's Network policy, not by this byte grammar.
func nodeNetworkShapes(fields map[string]json.RawMessage) bool {
	for _, name := range []string{"network_id", "profile_authority"} {
		if body, ok := fields[name]; ok && !exactNodeBytes(body) {
			return false
		}
	}
	if body, ok := fields["authorities"]; ok {
		var pins []json.RawMessage
		if json.Unmarshal(body, &pins) != nil {
			return false
		}
		for _, pin := range pins {
			if !exactNodeBytes(pin) {
				return false
			}
		}
	}
	return true
}

// exactNodeBytes refuses encoding/json's padded short and truncated overlong
// fixed arrays before a decoded value can become a trust declaration.
func exactNodeBytes(raw json.RawMessage) bool {
	var values []uint8
	return len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, &values) == nil && len(values) == 32
}

// nodeObject mirrors network/source/servingdeclaration.servingObject: exact field spelling, no
// duplicate or null member, one complete object. Unlike the Source serving
// declaration it admits omitted members (the node runtime grammar decodes them
// to zero); the allowed field set is still exact and closed per role.
func nodeObject(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	bad := errors.New("invalid node plan object")
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
	if err != nil || last != json.Delim('}') {
		return nil, bad
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, bad
	}
	return fields, nil
}
