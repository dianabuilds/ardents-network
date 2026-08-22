package nameresolution

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

const controlSchema = "ardents-private-name-control-v1"

type controlRequestWire struct {
	Schema    string           `json:"schema"`
	Operation controlOperation `json:"operation"`
	Admission namespace.Proof  `json:"admission"`
}

type controlResponseWire struct {
	Schema   string        `json:"schema"`
	Network  [32]byte      `json:"network"`
	Nonce    [32]byte      `json:"nonce"`
	Deadline int64         `json:"deadline"`
	Kind     string        `json:"kind"`
	Name     string        `json:"name"`
	Result   controlResult `json:"result"`
}

func controlDigest(operation controlOperation) ([32]byte, error) {
	declared := operation.OperationDigest
	if operation.Network != [32]byte{} || operation.Nonce != [32]byte{} || operation.Deadline != 0 ||
		declared == [32]byte{} || !validControlFields(operation, false) {
		return [32]byte{}, errors.New("private naming control operation is invalid")
	}
	operation.OperationDigest = [32]byte{}
	raw, err := json.Marshal(operation)
	if err != nil {
		return [32]byte{}, err
	}
	digest := sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
	if digest != declared {
		return [32]byte{}, errors.New("private naming control digest does not bind the operation")
	}
	return digest, nil
}

func controlRequest(operation controlOperation, admission namespace.Proof) ([]byte, error) {
	digest, err := dynamicControlDigest(operation)
	if err != nil || admission.Challenge.OperationDigest != digest {
		return nil, errors.New("private naming control request is invalid")
	}
	raw, err := json.Marshal(controlRequestWire{Schema: controlSchema, Operation: operation, Admission: admission})
	if err != nil {
		return nil, err
	}
	return padMessage(raw)
}

func decodeControlRequest(raw []byte) (controlOperation, namespace.Proof, error) {
	payload, err := unpadMessage(raw)
	if err != nil {
		return controlOperation{}, namespace.Proof{}, err
	}
	var wire controlRequestWire
	if err := decodeControlJSON(payload, &wire); err != nil || wire.Schema != controlSchema ||
		wire.Operation.OperationDigest == [32]byte{} || !validControlFields(wire.Operation, true) {
		return controlOperation{}, namespace.Proof{}, errors.New("private naming control request is invalid")
	}
	return wire.Operation, wire.Admission, nil
}

func controlResponse(operation controlOperation, result controlResult) ([]byte, error) {
	if !validControlResult(result) {
		return nil, errors.New("private naming control result is invalid")
	}
	raw, err := json.Marshal(controlResponseWire{Schema: controlSchema, Network: operation.Network,
		Nonce: operation.Nonce, Deadline: operation.Deadline, Kind: operation.Kind, Name: operation.Name, Result: result})
	if err != nil {
		return nil, err
	}
	return padMessage(raw)
}

func decodeControlResponse(raw []byte) (controlResponseWire, error) {
	payload, err := unpadMessage(raw)
	if err != nil {
		return controlResponseWire{}, err
	}
	var wire controlResponseWire
	if err := decodeControlJSON(payload, &wire); err != nil || wire.Schema != controlSchema ||
		wire.Network == [32]byte{} || wire.Nonce == [32]byte{} || wire.Deadline <= 0 ||
		wire.Kind == "" || wire.Name == "" || !validControlResult(wire.Result) {
		return controlResponseWire{}, errors.New("private naming control response is invalid")
	}
	return wire, nil
}

func decodeControlJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("private naming control JSON has trailing input")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("private naming control JSON is non-canonical")
	}
	return nil
}

func validControlResult(result controlResult) bool {
	return result.Class == "submitted" || result.Class == "denied"
}
