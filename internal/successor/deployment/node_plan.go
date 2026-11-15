package deployment

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// ErrNodePlan denotes invalid public deployment declarations. It does not
// represent an authenticated runtime decision.
var ErrNodePlan = errors.New("invalid Node deployment plan")

// JoinPlan is the closed public JOIN runtime declaration.
type JoinPlan struct {
	Schema            string            `json:"schema"`
	Network           NodeNetworkInputs `json:"network"`
	NodeID            [32]byte          `json:"node_id"`
	ProcessProfile    string            `json:"process_profile"`
	ParticipationRoot string            `json:"participation_root"`
	SpendRoot         string            `json:"spend_root"`
	HostingRoot       string            `json:"hosting_root"`
	Certificate       string            `json:"certificate"`
	PrivateKey        string            `json:"private_key"`
	Work              hosting.Traffic   `json:"work"`
	Termination       hosting.Traffic   `json:"termination"`
	RetirementWaitMS  uint32            `json:"retirement_wait_ms"`
}

// ForwardingPlan has the same fields as JOIN but retains its own role grammar.
type ForwardingPlan JoinPlan

// DescriptorPlan adds the independent Descriptor history root.
type DescriptorPlan struct {
	Schema            string            `json:"schema"`
	Network           NodeNetworkInputs `json:"network"`
	NodeID            [32]byte          `json:"node_id"`
	ProcessProfile    string            `json:"process_profile"`
	ParticipationRoot string            `json:"participation_root"`
	SpendRoot         string            `json:"spend_root"`
	HostingRoot       string            `json:"hosting_root"`
	DescriptorRoot    string            `json:"descriptor_root"`
	Certificate       string            `json:"certificate"`
	PrivateKey        string            `json:"private_key"`
	Work              hosting.Traffic   `json:"work"`
	Termination       hosting.Traffic   `json:"termination"`
	RetirementWaitMS  uint32            `json:"retirement_wait_ms"`
}

// IntroductionPlan adds the independent Introduction history root.
type IntroductionPlan struct {
	Schema            string            `json:"schema"`
	Network           NodeNetworkInputs `json:"network"`
	NodeID            [32]byte          `json:"node_id"`
	ProcessProfile    string            `json:"process_profile"`
	ParticipationRoot string            `json:"participation_root"`
	SpendRoot         string            `json:"spend_root"`
	HostingRoot       string            `json:"hosting_root"`
	IntroductionRoot  string            `json:"introduction_root"`
	Certificate       string            `json:"certificate"`
	PrivateKey        string            `json:"private_key"`
	Work              hosting.Traffic   `json:"work"`
	Termination       hosting.Traffic   `json:"termination"`
	RetirementWaitMS  uint32            `json:"retirement_wait_ms"`
}

// decodeNodePlan checks the original canonical encoding before role-specific
// checks. A declaration does not authenticate a runtime or open any resource.
func decodeNodePlan(raw []byte, plan any) bool {
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, plan) != nil {
		return false
	}
	canonical, err := json.Marshal(plan)
	return err == nil && bytes.Equal(raw, append(canonical, '\n'))
}

func validNodePlan(network NodeNetworkInputs, nodeID [32]byte, wait uint32, paths ...string) bool {
	return nodeID != [32]byte{} && ValidNodeTrustDeclaration(network.NetworkID, network.ProfileAuthority, network.Authorities, network.Threshold) && wait > 0 && wait <= 5000 && IndependentNodePaths(paths...)
}

// DecodeJoinPlan validates the fixed Join declaration without opening its inputs.
func DecodeJoinPlan(raw []byte) (JoinPlan, error) {
	var p JoinPlan
	if !decodeNodePlan(raw, &p) || p.Schema != "ardents-join-runtime-v1" || p.ProcessProfile != "ardents-join-process-v1" ||
		!validNodePlan(p.Network, p.NodeID, p.RetirementWaitMS, p.Network.Root, p.Network.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.Certificate, p.PrivateKey) {
		return JoinPlan{}, ErrNodePlan
	}
	return p, nil
}

// DecodeForwardingPlan validates the fixed Forwarding declaration without opening its inputs.
func DecodeForwardingPlan(raw []byte) (ForwardingPlan, error) {
	var p ForwardingPlan
	if !decodeNodePlan(raw, &p) || p.Schema != "ardents-forwarding-runtime-v1" || p.ProcessProfile != "ardents-forwarding-process-v1" ||
		!validNodePlan(p.Network, p.NodeID, p.RetirementWaitMS, p.Network.Root, p.Network.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.Certificate, p.PrivateKey) {
		return ForwardingPlan{}, ErrNodePlan
	}
	return p, nil
}

// DecodeDescriptorPlan validates the fixed Descriptor declaration without opening its inputs.
func DecodeDescriptorPlan(raw []byte) (DescriptorPlan, error) {
	var p DescriptorPlan
	if !decodeNodePlan(raw, &p) || p.Schema != "ardents-descriptor-runtime-v1" || p.ProcessProfile != "ardents-descriptor-process-v1" ||
		!validNodePlan(p.Network, p.NodeID, p.RetirementWaitMS, p.Network.Root, p.Network.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.DescriptorRoot, p.Certificate, p.PrivateKey) {
		return DescriptorPlan{}, ErrNodePlan
	}
	return p, nil
}

// DecodeIntroductionPlan validates the fixed Introduction declaration without opening its inputs.
func DecodeIntroductionPlan(raw []byte) (IntroductionPlan, error) {
	var p IntroductionPlan
	if !decodeNodePlan(raw, &p) || p.Schema != "ardents-introduction-runtime-v1" || p.ProcessProfile != "ardents-introduction-process-v1" ||
		!validNodePlan(p.Network, p.NodeID, p.RetirementWaitMS, p.Network.Root, p.Network.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.IntroductionRoot, p.Certificate, p.PrivateKey) {
		return IntroductionPlan{}, ErrNodePlan
	}
	return p, nil
}
