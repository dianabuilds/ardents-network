package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

func TestFailureUsesStructuredAPIError(t *testing.T) {
	connectErr := connect.NewError(connect.CodePermissionDenied, errors.New("fallback message"))
	detail, err := connect.NewErrorDetail(&protocol.Error{
		Code: "policy_rejected", Category: "policy", Message: "action denied",
		Domain: "workload", Operation: "start", Reason: "policy rule",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectErr.AddDetail(detail)
	var out bytes.Buffer
	Renderer{Err: &out}.Failure(connectErr)
	for _, expected := range []string{"error: policy_rejected", "domain: workload", "operation: start", "message: action denied"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("missing %q in %s", expected, out.String())
		}
	}
}
