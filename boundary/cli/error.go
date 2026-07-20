package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

type cliError struct {
	Code      string `json:"code,omitempty"`
	Category  string `json:"category,omitempty"`
	Message   string `json:"message"`
	Domain    string `json:"domain,omitempty"`
	Operation string `json:"operation,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

func renderError(w io.Writer, jsonMode bool, err error) {
	payload := buildCLIError(err)
	if jsonMode {
		renderErrorJSON(w, payload, err)
		return
	}
	renderErrorHuman(w, payload)
}

func buildCLIError(err error) cliError {
	payload := cliError{Message: err.Error()}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return payload
	}
	payload.Code = connectErr.Code().String()
	payload.Message = connectErr.Message()
	for _, detail := range connectErr.Details() {
		msg, valueErr := detail.Value()
		apiErr, ok := msg.(*ardentsv1.Error)
		if valueErr == nil && ok {
			payload.Code = apiErr.GetCode()
			payload.Category = apiErr.GetCategory()
			payload.Message = apiErr.GetMessage()
			payload.Domain = apiErr.GetDomain()
			payload.Operation = apiErr.GetOperation()
			payload.Reason = apiErr.GetReason()
			payload.Retryable = apiErr.GetRetryable()
			break
		}
	}
	return payload
}

func renderErrorJSON(w io.Writer, payload cliError, err error) {
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		_, _ = fmt.Fprintf(w, "{\"message\":%q}\n", err.Error())
		return
	}
	_, _ = fmt.Fprintln(w, string(data))
}

func renderErrorHuman(w io.Writer, payload cliError) {
	if payload.Code != "" {
		_, _ = fmt.Fprintf(w, "error: %s\n", payload.Code)
	}
	if payload.Category != "" {
		_, _ = fmt.Fprintf(w, "category: %s\n", payload.Category)
	}
	if payload.Domain != "" {
		_, _ = fmt.Fprintf(w, "domain: %s\n", payload.Domain)
	}
	if payload.Operation != "" {
		_, _ = fmt.Fprintf(w, "operation: %s\n", payload.Operation)
	}
	_, _ = fmt.Fprintf(w, "message: %s\n", payload.Message)
	if payload.Reason != "" {
		_, _ = fmt.Fprintf(w, "reason: %s\n", payload.Reason)
	}
	if payload.Retryable {
		_, _ = fmt.Fprintln(w, "retryable: true")
	}
}
