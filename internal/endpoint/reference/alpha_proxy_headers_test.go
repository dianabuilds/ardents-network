package reference

import (
	"net/http"
	"testing"
)

func TestCloneForwardHeadersDoesNotSendBrowserEntryAuthenticationToOrigin(t *testing.T) {
	headers := http.Header{
		"Accept":              {"text/html"},
		"Proxy-Authorization": {"Basic local-browser-entry-password"},
	}
	forwarded := cloneForwardHeaders(headers)
	if forwarded.Get("Proxy-Authorization") != "" {
		t.Fatalf("forwarded Browser Entry authentication = %q", forwarded.Get("Proxy-Authorization"))
	}
	if forwarded.Get("Accept") != "text/html" {
		t.Fatalf("forwarded ordinary header = %q, want retained value", forwarded.Get("Accept"))
	}
}
