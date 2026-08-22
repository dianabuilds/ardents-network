package nameresolution

import (
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
	control, decodeErr := decodeControlRequest(fixed)
	now := gateway.config.Clock()
	if err != nil || decodeErr != nil || control.binding.network != gateway.records.network ||
		control.binding.deadline <= now.UnixNano() || control.binding.deadline > now.Add(15*time.Second).UnixNano() ||
		control.admission.Challenge.Node != gateway.config.NodeID || control.admission.Challenge.Network != control.binding.network ||
		control.admission.Challenge.OperationDigest != control.submission.Digest() ||
		!gateway.acceptNonce(control.binding.nonce, control.binding.deadline, now) {
		gateway.reject(writer)
		return
	}
	class, _, _, _ := gateway.state.authority.Apply(control.submission.Canonical(), control.admission)
	result := controlResult{Class: "denied"}
	if class == "accepted" {
		result.Class = "submitted"
	}
	response, err := controlResponse(control.binding, control.submission.Digest(), result)
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

// ControlObservation returns only bounded counts, never operation fields.
func (gateway *gateway) ControlObservation() (requests, accepted, denied uint32) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	return gateway.observation.ControlRequests, gateway.observation.ControlAccepted, gateway.observation.ControlDenied
}
