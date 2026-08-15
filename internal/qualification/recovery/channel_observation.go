package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
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

func verifyCommonChannelEvidence(raw []byte, cell Cell, scope hostScopeEvidence) (commonChannelEvidence, error) {
	var value commonChannelEvidence
	if err := decodeCanonicalChannelEvidence(raw, &value); err != nil {
		return value, err
	}
	if !validChannelObservation(value.Initial, scope) || !validChannelObservation(value.Replacement, scope) ||
		value.Initial.State != "established" || value.Replacement.State != "established" ||
		value.Initial.Ref.NetworkScope != value.Replacement.Ref.NetworkScope ||
		value.Initial.Ref.Commitment == value.Replacement.Ref.Commitment ||
		value.Initial.Ref.Incarnation == value.Replacement.Ref.Incarnation {
		return value, errors.New("common Carrier channel observations are invalid")
	}
	if value.Initial.Ref.Local != cell.InitialCarrierLocal || value.Initial.Ref.Remote != cell.InitialCarrierRemote ||
		value.Initial.Ref.Incarnation != cell.InitialCarrier ||
		value.Replacement.Ref.Local != cell.ReplacementCarrierLocal ||
		value.Replacement.Ref.Remote != cell.ReplacementCarrierRemote ||
		value.Replacement.Ref.Incarnation != cell.ReplacementCarrier {
		return value, errors.New("common Carrier channel observations differ from the retained cell")
	}
	if !validChannelFault(value.Fault, scope) || value.Fault.Resource != value.Initial.Ref ||
		value.Fault.Operation != "retire-channel" || value.Fault.Postcondition != "unavailable" ||
		value.Fault.ObservedAtNanos != value.Fault.InvocationCompletedNanos {
		return value, errors.New("common Carrier fault receipt is invalid")
	}
	if !validChannelState(value.Retirement, scope) || value.Retirement.Resource != value.Initial.Ref ||
		value.Retirement.State != "retired" || value.Initial.ObservedAtNanos < cell.HostStartedAtNanos ||
		value.Initial.ObservedAtNanos > value.Fault.InvocationStartedNanos ||
		value.Retirement.ObservedAtNanos < value.Fault.InvocationStartedNanos ||
		value.Retirement.ObservedAtNanos > value.Fault.InvocationCompletedNanos ||
		value.Replacement.ObservedAtNanos < value.Fault.InvocationCompletedNanos ||
		value.Replacement.ObservedAtNanos > cell.HostCompletedAtNanos {
		return value, errors.New("common Carrier state chronology is invalid")
	}
	return value, nil
}

func decodeCanonicalChannelEvidence(raw []byte, value *commonChannelEvidence) error {
	if len(raw) == 0 || len(raw) > 128<<10 {
		return errors.New("common Carrier channel evidence is malformed")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("common Carrier channel evidence contains multiple values")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !canonicalJSONEqual(raw, canonical) {
		return errors.Join(err, errors.New("common Carrier channel evidence is not canonical"))
	}
	return nil
}

func validChannelObservation(value channelObservationEvidence, scope hostScopeEvidence) bool {
	return validChannelRef(value.Ref, scope) && value.ObservedAtNanos > 0 && value.State != "" &&
		boundedProjection(value.AdapterProjection) && value.Commitment == channelObservationCommitment(value)
}

func validChannelFault(value channelFaultEvidence, scope hostScopeEvidence) bool {
	return validChannelRef(value.Resource, scope) && value.Operation != "" && value.Postcondition != "" &&
		value.InvocationStartedNanos > 0 && value.InvocationCompletedNanos >= value.InvocationStartedNanos &&
		value.ObservedAtNanos >= value.InvocationStartedNanos &&
		value.ObservedAtNanos <= value.InvocationCompletedNanos && boundedProjection(value.AdapterProjection) &&
		value.Commitment == channelFaultCommitment(value)
}

func validChannelState(value channelStateEvidence, scope hostScopeEvidence) bool {
	return validChannelRef(value.Resource, scope) && value.State != "" && value.ObservedAtNanos > 0 &&
		boundedProjection(value.AdapterProjection) && value.Commitment == channelStateCommitment(value)
}

func validChannelRef(value channelRefEvidence, scope hostScopeEvidence) bool {
	return value.Adapter == scope.Adapter && value.Scope == scope.Commitment && value.NetworkScope != [32]byte{} &&
		value.Family == "tcp" && validChannelEndpoint(value.Local) && validChannelEndpoint(value.Remote) && value.Incarnation != "" &&
		len(value.Incarnation) <= 256 && value.Commitment == channelRefCommitment(value)
}

func validChannelEndpoint(value string) bool {
	host, portText, err := net.SplitHostPort(value)
	port, parseErr := strconv.ParseUint(portText, 10, 16)
	return err == nil && parseErr == nil && port > 0 && net.ParseIP(host) != nil
}

func boundedProjection(raw []byte) bool { return len(raw) > 0 && len(raw) <= 64<<10 }

func channelRefCommitment(value channelRefEvidence) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-channel-ref-v1\x00" + value.Adapter + "\x00"))
	_, _ = hash.Write(value.Scope[:])
	_, _ = hash.Write(value.NetworkScope[:])
	_, _ = hash.Write([]byte(value.Family + "\x00" + value.Local + "\x00" + value.Remote + "\x00" + value.Incarnation))
	return sum32(hash)
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
	digest := sha256.Sum256(compactJSON(projection))
	_, _ = hash.Write(digest[:])
	return sum32(hash)
}

func sum32(hash interface{ Sum([]byte) []byte }) [32]byte {
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
