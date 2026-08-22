package nameresolution

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func (gateway *gateway) control(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || gateway.state.authority == nil {
		gateway.reject(writer)
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(request.Body, fixedMessageSize+1))
	operation, admission, decodeErr := decodeControlRequest(fixed)
	now := gateway.config.Clock()
	digest, digestErr := dynamicControlDigest(operation)
	if err != nil || decodeErr != nil || digestErr != nil || operation.Network != gateway.records.network ||
		operation.Deadline <= now.UnixNano() || operation.Deadline > now.Add(15*time.Second).UnixNano() ||
		admission.Challenge.Node != gateway.config.NodeID || admission.Challenge.Network != operation.Network ||
		admission.Challenge.OperationDigest != digest || !gateway.acceptNonce(operation.Nonce, operation.Deadline, now) {
		gateway.reject(writer)
		return
	}
	authorityInput, encodeErr := authorityOperation(operation)
	if encodeErr != nil {
		gateway.reject(writer)
		return
	}
	class, _, _, _ := gateway.state.authority.Apply(authorityInput, admission)
	result := controlResult{Class: "denied"}
	if class == "accepted" {
		result.Class = "submitted"
	}
	response, err := controlResponse(operation, result)
	if err != nil {
		gateway.reject(writer)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(response)
	gateway.mu.Lock()
	gateway.observation.ControlRequests++
	if result.Class == "submitted" {
		gateway.observation.ControlAccepted++
	} else {
		gateway.observation.ControlDenied++
	}
	gateway.mu.Unlock()
}

func authorityOperation(operation controlOperation) ([]byte, error) {
	operation.Network, operation.Nonce, operation.Deadline = [32]byte{}, [32]byte{}, 0
	if _, err := controlDigest(operation); err != nil {
		return nil, err
	}
	return json.Marshal(operation)
}

func dynamicControlDigest(operation controlOperation) ([32]byte, error) {
	if operation.Network == [32]byte{} || operation.Nonce == [32]byte{} || operation.Deadline <= 0 {
		return [32]byte{}, errors.New("private naming control binding is invalid")
	}
	operation.Network, operation.Nonce, operation.Deadline = [32]byte{}, [32]byte{}, 0
	return controlDigest(operation)
}

// ControlObservation returns only bounded counts, never operation fields.
func (gateway *gateway) ControlObservation() (requests, accepted, denied uint32) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	return gateway.observation.ControlRequests, gateway.observation.ControlAccepted, gateway.observation.ControlDenied
}
