package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

type channelRefEvidence struct {
	Adapter, Family, Local, Remote, Incarnation string
	Scope, NetworkScope, Commitment             [32]byte
}

type channelObservationEvidence struct {
	Ref               channelRefEvidence
	State             string
	ObservedAtNanos   int64
	AdapterProjection json.RawMessage
	Commitment        [32]byte
}

type channelFaultEvidence struct {
	Resource                                         channelRefEvidence
	Operation, Postcondition                         string
	InvocationStartedNanos, InvocationCompletedNanos int64
	ObservedAtNanos                                  int64
	AdapterProjection                                json.RawMessage
	Commitment                                       [32]byte
}

type channelStateEvidence struct {
	Resource          channelRefEvidence
	State             string
	ObservedAtNanos   int64
	AdapterProjection json.RawMessage
	Commitment        [32]byte
}

type commonChannelEvidence struct {
	Initial, Replacement channelObservationEvidence
	Fault                channelFaultEvidence
	Retirement           channelStateEvidence
}

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

func freezeCommonChannelEvidence(scope hostScopeEvidence, network, hostProcess, controller string,
	initial, replacement carrierObservation, fault carrierFaultOutcome,
	initialObservedAt, replacementObservedAt int64) ([]byte, error) {
	if scope.Adapter == "" || scope.Commitment == [32]byte{} || network == "" || hostProcess == "" || controller == "" ||
		initial.SocketIDSHA256 == "" || initial.LocalAddress == "" || initial.RemoteAddress == "" ||
		replacement.SocketIDSHA256 == "" || replacement.LocalAddress == "" || replacement.RemoteAddress == "" ||
		initial.SocketIDSHA256 == replacement.SocketIDSHA256 || initialObservedAt <= 0 ||
		fault.hostFaultAt < initialObservedAt || fault.hostCompletedAt < fault.hostFaultAt ||
		fault.hostRetiredAt < fault.hostFaultAt || fault.hostRetiredAt > fault.hostCompletedAt ||
		replacementObservedAt < fault.hostCompletedAt || fault.commitment != initial.SocketIDSHA256 ||
		fault.retiredCommitment != initial.SocketIDSHA256 || fault.retiredAfter <= 0 ||
		!fault.controllerRemoved || !fault.resourceAbsent || !fault.socketRetired {
		return nil, errors.New("common channel observation input is incomplete or inconsistent")
	}
	initialProjection, err := json.Marshal(dockerChannelProjection{Project: scope.AdapterProjection,
		Network: network, HostProcess: hostProcess, Interface: initial.InterfaceName,
		SocketCommitment: initial.SocketIDSHA256, InterfaceIndex: initial.InterfaceIndex, Inode: initial.Inode})
	if err != nil {
		return nil, fmt.Errorf("encode initial Docker channel projection: %w", err)
	}
	replacementProjection, err := json.Marshal(dockerChannelProjection{Project: scope.AdapterProjection,
		Network: network, HostProcess: hostProcess, Interface: replacement.InterfaceName,
		SocketCommitment: replacement.SocketIDSHA256, InterfaceIndex: replacement.InterfaceIndex, Inode: replacement.Inode})
	if err != nil {
		return nil, fmt.Errorf("encode replacement Docker channel projection: %w", err)
	}
	initialRef := observedChannelRef(scope, network, initial)
	replacementRef := observedChannelRef(scope, network, replacement)
	value := commonChannelEvidence{
		Initial: channelObservationEvidence{Ref: initialRef, State: "established",
			ObservedAtNanos: initialObservedAt, AdapterProjection: initialProjection},
		Replacement: channelObservationEvidence{Ref: replacementRef, State: "established",
			ObservedAtNanos: replacementObservedAt, AdapterProjection: replacementProjection},
		Fault: channelFaultEvidence{Resource: initialRef, Operation: "retire-channel", Postcondition: "unavailable",
			InvocationStartedNanos: fault.hostFaultAt, InvocationCompletedNanos: fault.hostCompletedAt,
			ObservedAtNanos: fault.hostCompletedAt},
		Retirement: channelStateEvidence{Resource: initialRef, State: "retired",
			ObservedAtNanos: fault.hostRetiredAt},
	}
	value.Fault.AdapterProjection, err = json.Marshal(dockerChannelFaultProjection{Project: scope.AdapterProjection,
		Controller: controller, Network: network, Interface: initial.InterfaceName,
		ControllerRemoved: fault.controllerRemoved, Absent: fault.resourceAbsent,
		CutAfterNanos: fault.cutAfter, AbsenceAfterNanos: fault.absenceAfter})
	if err != nil {
		return nil, fmt.Errorf("encode Docker channel fault projection: %w", err)
	}
	value.Retirement.AdapterProjection, err = json.Marshal(dockerChannelStateProjection{Project: scope.AdapterProjection,
		SocketCommitment: fault.retiredCommitment, LeftEstablishedAfterNanos: fault.retiredAfter})
	if err != nil {
		return nil, fmt.Errorf("encode Docker channel state projection: %w", err)
	}
	value.Initial.Commitment = channelObservationCommitment(value.Initial)
	value.Replacement.Commitment = channelObservationCommitment(value.Replacement)
	value.Fault.Commitment = channelFaultCommitment(value.Fault)
	value.Retirement.Commitment = channelStateCommitment(value.Retirement)
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode common channel evidence: %w", err)
	}
	return raw, nil
}

func observedChannelRef(scope hostScopeEvidence, network string, value carrierObservation) channelRefEvidence {
	result := channelRefEvidence{Adapter: scope.Adapter, Scope: scope.Commitment,
		NetworkScope: channelNetworkScope(scope, network), Family: "tcp", Local: value.LocalAddress,
		Remote: value.RemoteAddress, Incarnation: value.SocketIDSHA256}
	result.Commitment = channelRefCommitment(result)
	return result
}

func channelNetworkScope(scope hostScopeEvidence, network string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-docker-channel-scope-v1\x00"))
	_, _ = hash.Write(scope.Commitment[:])
	_, _ = hash.Write([]byte(scope.AdapterProjection + "\x00" + network))
	return channelSum32(hash)
}

func channelRefCommitment(value channelRefEvidence) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-channel-ref-v1\x00" + value.Adapter + "\x00"))
	_, _ = hash.Write(value.Scope[:])
	_, _ = hash.Write(value.NetworkScope[:])
	_, _ = hash.Write([]byte(value.Family + "\x00" + value.Local + "\x00" + value.Remote + "\x00" + value.Incarnation))
	return channelSum32(hash)
}

func channelObservationCommitment(value channelObservationEvidence) [32]byte {
	return channelEventCommitment("observation", value.Ref.Commitment, value.State, "",
		value.ObservedAtNanos, 0, 0, value.AdapterProjection)
}

func channelFaultCommitment(value channelFaultEvidence) [32]byte {
	return channelEventCommitment("fault", value.Resource.Commitment, value.Operation, value.Postcondition,
		value.ObservedAtNanos, value.InvocationStartedNanos, value.InvocationCompletedNanos, value.AdapterProjection)
}

func channelStateCommitment(value channelStateEvidence) [32]byte {
	return channelEventCommitment("state", value.Resource.Commitment, value.State, "",
		value.ObservedAtNanos, 0, 0, value.AdapterProjection)
}

func channelEventCommitment(kind string, subject [32]byte, first, second string,
	observed, started, completed int64, projection []byte) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-channel-" + kind + "-v1\x00"))
	_, _ = hash.Write(subject[:])
	_, _ = hash.Write([]byte(first + "\x00" + second + "\x00"))
	var encoded [24]byte
	binary.BigEndian.PutUint64(encoded[:8], uint64(observed))
	binary.BigEndian.PutUint64(encoded[8:16], uint64(started))
	binary.BigEndian.PutUint64(encoded[16:], uint64(completed))
	_, _ = hash.Write(encoded[:])
	digest := sha256.Sum256(projection)
	_, _ = hash.Write(digest[:])
	return channelSum32(hash)
}

func channelSum32(hash interface{ Sum([]byte) []byte }) [32]byte {
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
