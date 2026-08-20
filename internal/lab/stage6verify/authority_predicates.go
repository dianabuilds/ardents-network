package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type recoveryTrace struct {
	Network          string   `json:"network"`
	Name             string   `json:"name"`
	Generation       uint64   `json:"generation"`
	PolicyRevision   uint64   `json:"policy_revision"`
	CurrentAuthority string   `json:"current_authority"`
	Threshold        uint8    `json:"threshold"`
	Participants     []string `json:"participants"`
	DelayMillis      int64    `json:"delay_millis"`
	PolicyDigest     string   `json:"policy_digest"`
	OperationID      string   `json:"operation_id"`
	Successor        string   `json:"successor"`
	StartedAt        int64    `json:"started_at"`
	CompletesAt      int64    `json:"completes_at"`
	Signatures       []string `json:"signatures"`
}

func verifyAuthorityTrace(trace traceRecord) bool {
	before, err := decodeRecords(trace.Input)
	if err != nil || len(before) != 1 {
		return false
	}
	after, err := decodeRecords(trace.Output)
	if err != nil {
		return false
	}
	if trace.Cell == "B3" {
		return verifyRecoveryTrace(before[0], after, trace.Auxiliary)
	}
	fieldOffset := 0
	if trace.Cell == "B5" {
		if len(trace.Fields) != 4 || trace.Fields[0] != "stale-proof" {
			return false
		}
		fieldOffset = 1
	} else if len(trace.Fields) != 3 {
		return false
	}
	if len(after) != 1 || after[0].Generation != before[0].Generation ||
		after[0].Revision != before[0].Revision+1 || after[0].Authority != trace.Fields[fieldOffset+1] ||
		after[0].Authority == before[0].Authority || after[0].Target != before[0].Target {
		return false
	}
	network, err := decodeFixedHex(trace.Fields[fieldOffset], 32)
	if err != nil || trace.Fields[fieldOffset+2] != map[string]string{"B0": "rotate", "B1": "transfer", "B5": "rotate"}[trace.Cell] {
		return false
	}
	public, err := decodeFixedHex(before[0].Authority, ed25519.PublicKeySize)
	if err != nil || len(trace.Auxiliary) != ed25519.SignatureSize {
		return false
	}
	transcript := authorityTransitionTranscript(network, before[0], trace.Fields[fieldOffset+2], after[0].Authority)
	return ed25519.Verify(ed25519.PublicKey(public), transcript, trace.Auxiliary)
}

func authorityTransitionTranscript(network []byte, current decodedRecord, kind, successor string) []byte {
	out := appendText64(nil, "ardents-name-authority-transition-v1")
	out = append(out, network...)
	out = appendBytes64(out, encodeRecord(current))
	out = appendText64(out, kind)
	out = appendText64(out, current.Name)
	out = binary.BigEndian.AppendUint64(out, 0)
	out = binary.BigEndian.AppendUint32(out, 0)
	out = binary.BigEndian.AppendUint64(out, current.Generation)
	out = binary.BigEndian.AppendUint64(out, current.Revision)
	out = appendText64(out, current.Authority)
	out = appendText64(out, successor)
	out = append(out, make([]byte, 32)...)
	for range 3 {
		out = binary.BigEndian.AppendUint64(out, 0)
	}
	out = append(out, make([]byte, 32)...)
	for range 4 {
		out = binary.BigEndian.AppendUint64(out, 0)
	}
	out = append(out, make([]byte, 32)...)
	out = binary.BigEndian.AppendUint64(out, 0)
	out = append(out, make([]byte, 32+32)...)
	for range 2 {
		out = binary.BigEndian.AppendUint64(out, 0)
	}
	out = append(out, 0, 0)
	return out
}

func verifyRecoveryTrace(before decodedRecord, after []decodedRecord, raw []byte) bool {
	var evidence recoveryTrace
	if err := decodeNestedJSON(raw, &evidence); err != nil || len(after) != 3 || evidence.Threshold != 2 ||
		len(evidence.Participants) != 2 || len(evidence.Signatures) != 2 || evidence.Name != before.Name ||
		evidence.Generation != before.Generation || evidence.PolicyRevision != before.RecoveryPolicyRev ||
		evidence.DelayMillis < 72*60*60*1_000 || evidence.DelayMillis > 30*24*60*60*1_000 ||
		evidence.CompletesAt-evidence.StartedAt != evidence.DelayMillis {
		return false
	}
	network, err := decodeFixedHex(evidence.Network, 32)
	if err != nil {
		return false
	}
	current, err := decodeFixedHex(evidence.CurrentAuthority, 32)
	if err != nil || evidence.CurrentAuthority != before.Authority {
		return false
	}
	participants := make([][]byte, len(evidence.Participants))
	for index, rawParticipant := range evidence.Participants {
		participants[index], err = decodeFixedHex(rawParticipant, 32)
		if err != nil || bytes.Equal(participants[index], current) ||
			index > 0 && bytes.Compare(participants[index-1], participants[index]) >= 0 {
			return false
		}
	}
	policyDigest := recoveryPolicyDigest(network, evidence, current, participants)
	if hex.EncodeToString(policyDigest[:]) != evidence.PolicyDigest || policyDigest != before.RecoveryPolicy {
		return false
	}
	operationID, err := decodeFixedHex(evidence.OperationID, 32)
	if err != nil {
		return false
	}
	successor, err := decodeFixedHex(evidence.Successor, 32)
	if err != nil {
		return false
	}
	transcript := recoveryTranscript(network, evidence, policyDigest, operationID, successor)
	for index, rawSignature := range evidence.Signatures {
		signature, decodeErr := decodeFixedHex(rawSignature, ed25519.SignatureSize)
		if decodeErr != nil || !ed25519.Verify(ed25519.PublicKey(participants[index]), transcript, signature) {
			return false
		}
	}
	pending, completed, resumed := after[0], after[1], after[2]
	var operation, successorArray [32]byte
	copy(operation[:], operationID)
	copy(successorArray[:], successor)
	return pending.Recovery == "recovery-pending" && pending.Revision == before.Revision+1 &&
		pending.RecoveryOperation == operation && pending.RecoverySuccessor == successorArray &&
		pending.RecoveryStarted == evidence.StartedAt && pending.RecoveryExpires == evidence.CompletesAt &&
		completed.Revision == pending.Revision+1 && completed.Recovery == "stable" &&
		completed.Consistency == "unavailable" && completed.Target == [32]byte{} && completed.Authority == evidence.Successor &&
		resumed.Revision == completed.Revision+1 && resumed.Authority == completed.Authority &&
		resumed.Consistency == "current" && resumed.Target == [32]byte{9}
}

func recoveryPolicyDigest(network []byte, evidence recoveryTrace, current []byte, participants [][]byte) [32]byte {
	wire := canonicalNameWire(evidence.Name)
	out := appendText32(nil, "ardents-name-recovery-policy-v1")
	out = append(out, network...)
	out = appendBytes32(out, wire)
	out = binary.BigEndian.AppendUint64(out, evidence.Generation)
	out = binary.BigEndian.AppendUint64(out, evidence.PolicyRevision)
	out = append(out, current...)
	out = append(out, evidence.Threshold, byte(len(participants)))
	for _, participant := range participants {
		out = append(out, participant...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(evidence.DelayMillis))
	return sha256.Sum256(out)
}

func recoveryTranscript(network []byte, evidence recoveryTrace, policy [32]byte, operation, successor []byte) []byte {
	out := appendText32(nil, "ardents-name-recovery-initiate-v1")
	out = append(out, network...)
	out = appendBytes32(out, canonicalNameWire(evidence.Name))
	out = binary.BigEndian.AppendUint64(out, evidence.Generation)
	out = append(out, policy[:]...)
	out = append(out, operation...)
	out = append(out, successor...)
	out = binary.BigEndian.AppendUint64(out, uint64(evidence.StartedAt))
	return binary.BigEndian.AppendUint64(out, uint64(evidence.CompletesAt))
}

func decodeNestedJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("nested evidence is non-canonical")
	}
	return nil
}

func decodeFixedHex(raw string, size int) ([]byte, error) {
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != size || hex.EncodeToString(decoded) != raw {
		return nil, errors.New("hex value is non-canonical")
	}
	return decoded, nil
}

func canonicalNameWire(name string) []byte {
	out := binary.BigEndian.AppendUint16(nil, 1)
	for _, label := range bytes.Split([]byte(name), []byte{'.'}) {
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	return out
}

func appendText32(out []byte, value string) []byte { return appendBytes32(out, []byte(value)) }
func appendBytes32(out, value []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
func appendBytes64(out, value []byte) []byte {
	out = binary.BigEndian.AppendUint64(out, uint64(len(value)))
	return append(out, value...)
}
