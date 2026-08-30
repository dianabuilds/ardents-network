package reference

import (
	"net/http"
	"testing"
)

func TestCloneForwardHeadersDoesNotSendBrowserEntryAuthenticationToOrigin(t *testing.T) {
	headers := http.Header{
		"Accept":              {"text/html"},
		"Proxy-Authorization": {"Basic secret"},
	}
	forwarded := cloneForwardHeaders(headers)
	if forwarded.Get("Proxy-Authorization") != "" {
		t.Fatal("forwarded headers disclose Browser Entry proxy authentication")
	}
	if forwarded.Get("Accept") != "text/html" {
		t.Fatalf("forwarded ordinary header = %q, want retained value", forwarded.Get("Accept"))
	}
}
