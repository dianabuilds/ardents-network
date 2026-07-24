package output

import (
	"bytes"
	"encoding/json"
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

func TestProtoJSONV1IsDeterministicAndEmitsUnpopulatedFields(t *testing.T) {
	message := &protocol.NodeStatusResponse{}
	var first, second bytes.Buffer
	NewRenderer(&first, &bytes.Buffer{}, true).Message(message)
	NewRenderer(&second, &bytes.Buffer{}, true).Message(message)

	if first.String() != second.String() {
		t.Fatalf("proto JSON is not deterministic:\nfirst=%s\nsecond=%s", first.String(), second.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(first.Bytes(), &decoded); err != nil {
		t.Fatalf("proto JSON is invalid: %v\n%s", err, first.String())
	}
	if _, ok := decoded["status"]; !ok {
		t.Fatalf("proto JSON must retain EmitUnpopulated fields: %s", first.String())
	}
}

func TestJSONLinesV1WritesOneValidDocumentPerEvent(t *testing.T) {
	var out bytes.Buffer
	for range 2 {
		if err := JSONLine(&out, &protocol.EventEnvelope{}); err != nil {
			t.Fatal(err)
		}
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("JSON Lines count = %d, want 2: %q", len(lines), out.String())
	}
	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
	}
}

func TestJSONFailureUsesCommonErrorObjectOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := NewRenderer(&stdout, &stderr, true).Failure(connect.NewError(connect.CodeUnavailable, errors.New("offline")))
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("code = %d, stdout = %q", code, stdout.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &decoded); err != nil {
		t.Fatalf("failure output is not JSON: %v: %s", err, stderr.String())
	}
	if decoded["code"] != "unavailable" || decoded["message"] != "offline" {
		t.Fatalf("unexpected failure object: %#v", decoded)
	}
}
