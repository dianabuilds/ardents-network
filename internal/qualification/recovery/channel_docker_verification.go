package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type dockerChannelProjection struct {
	Project, Network, HostProcess, Interface, SocketCommitment string
	InterfaceIndex                                             int
	Inode                                                      uint32
}

type dockerChannelFaultProjection struct {
	Project, Controller, Network, Interface string
	ControllerRemoved, Absent               bool
	CutAfterNanos, AbsenceAfterNanos        int64
}

type dockerChannelStateProjection struct {
	Project, SocketCommitment string
	LeftEstablishedAfterNanos int64
	Established               bool
}

func validDockerChannelEvidence(value commonChannelEvidence, cell Cell, scope hostScopeEvidence) error {
	initial := dockerChannelProjection{Project: scope.AdapterProjection, Network: cell.FaultNetwork,
		HostProcess: cell.FaultContainer, Interface: cell.InitialCarrierInterface,
		SocketCommitment: cell.InitialCarrier, InterfaceIndex: cell.InitialCarrierInterfaceIndex,
		Inode: cell.InitialCarrierInode}
	replacement := dockerChannelProjection{Project: scope.AdapterProjection, Network: cell.FaultNetwork,
		HostProcess: cell.FaultContainer, Interface: cell.ReplacementCarrierInterface,
		SocketCommitment: cell.ReplacementCarrier, InterfaceIndex: cell.ReplacementCarrierInterfaceIndex,
		Inode: cell.ReplacementCarrierInode}
	fault := dockerChannelFaultProjection{Project: scope.AdapterProjection, Controller: cell.FaultController,
		Network: cell.FaultNetwork, Interface: cell.InitialCarrierInterface,
		ControllerRemoved: cell.FaultControllerRemoved, Absent: cell.FaultResourceAbsent,
		CutAfterNanos: cell.CarrierCutAfterNanos, AbsenceAfterNanos: cell.AbsenceAfterNanos}
	var retirement dockerChannelStateProjection
	if !canonicalProjection(value.Initial.AdapterProjection, initial) ||
		!canonicalProjection(value.Replacement.AdapterProjection, replacement) ||
		!canonicalProjection(value.Fault.AdapterProjection, fault) ||
		!decodeCanonicalProjection(value.Retirement.AdapterProjection, &retirement) ||
		retirement.Project != scope.AdapterProjection || retirement.SocketCommitment != cell.RetiredCarrier ||
		retirement.LeftEstablishedAfterNanos <= 0 || retirement.LeftEstablishedAfterNanos > int64(5*time.Second) ||
		retirement.Established ||
		value.Initial.Ref.NetworkScope != dockerChannelNetworkScope(scope, initial.Network) ||
		value.Replacement.Ref.NetworkScope != dockerChannelNetworkScope(scope, replacement.Network) {
		return errors.New("Docker Carrier channel projection is invalid")
	}
	return nil
}

func canonicalProjection[T any](raw []byte, expected T) bool {
	var value T
	if !decodeCanonicalProjection(raw, &value) {
		return false
	}
	want, err := json.Marshal(expected)
	return err == nil && canonicalJSONEqual(raw, want)
}

func decodeCanonicalProjection(raw []byte, value any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	canonical, err := json.Marshal(value)
	return err == nil && canonicalJSONEqual(raw, canonical)
}

func dockerChannelNetworkScope(scope hostScopeEvidence, network string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-docker-channel-scope-v1\x00"))
	_, _ = hash.Write(scope.Commitment[:])
	_, _ = hash.Write([]byte(scope.AdapterProjection + "\x00" + network))
	return sum32(hash)
}
