package resolution

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
	if err != nil || decodeErr != nil || control.binding.network != gateway.state.namespace.Network() ||
		control.binding.deadline <= now.UnixNano() || control.binding.deadline > now.Add(15*time.Second).UnixNano() ||
		control.admission.Challenge.Node != gateway.config.NodeID || control.admission.Challenge.Network != control.binding.network ||
		control.admission.Challenge.OperationDigest != control.submission.Digest() ||
		!gateway.acceptNonce(control.binding.nonce, control.binding.deadline, now) {
		gateway.reject(writer)
		return
	}
	class := gateway.state.authority.Submit(control.submission, control.admission)
	result := controlResult{Class: "denied"}
	if class == "submitted" {
		result.Class = "submitted"
	}
	response, err := controlResponse(control.binding, control.submission.Digest(), result)
	if err != nil {
		gateway.reject(writer)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(response)
}
