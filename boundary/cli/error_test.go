package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func TestBuildCLIErrorUsesStructuredAPIErrorDetail(t *testing.T) {
	connectErr := connect.NewError(connect.CodePermissionDenied, errors.New("fallback message"))
	detail, err := connect.NewErrorDetail(&ardentsv1.Error{
		Code:      "policy_rejected",
		Category:  "policy",
		Message:   "action denied",
		Domain:    "workload",
		Operation: "start",
		Reason:    "policy rule",
		Retryable: false,
	})
	if err != nil {
		t.Fatalf("NewErrorDetail() error = %v", err)
	}
	connectErr.AddDetail(detail)

	payload := buildCLIError(connectErr)
	if payload.Code != "policy_rejected" || payload.Domain != "workload" || payload.Operation != "start" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Message != "action denied" || payload.Reason != "policy rule" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestRenderErrorHumanPrintsStructuredFields(t *testing.T) {
	var out bytes.Buffer
	renderErrorHuman(&out, cliError{
		Code:      "not_found",
		Category:  "input",
		Message:   "missing resource",
		Domain:    "data",
		Operation: "get",
		Reason:    "unknown blob",
		Retryable: true,
	})

	text := out.String()
	for _, want := range []string{
		"error: not_found",
		"category: input",
		"domain: data",
		"operation: get",
		"message: missing resource",
		"reason: unknown blob",
		"retryable: true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
}
