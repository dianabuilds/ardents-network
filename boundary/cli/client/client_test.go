package client

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRequestAddsBearerAuthorization(t *testing.T) {
	c := New(Config{
		BaseURL: "http://127.0.0.1:8080",
		Token:   "test-token",
		Timeout: time.Second,
	})
	req := Request(c.token, &struct{}{})
	if got := req.Header().Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestContextTransportAddsServerEnforcedBindingsAndScopes(t *testing.T) {
	var captured *http.Request
	transport := contextTransport{
		base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			captured = request
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
		expectedNode: "node-a", expectedPrincipal: "p_a", scopes: []string{"node.status", "diagnostics.snapshot"},
	}
	request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1/rpc", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	if got := captured.Header.Get("Ardents-Expected-Node"); got != "node-a" {
		t.Fatalf("expected node = %q", got)
	}
	if got := captured.Header.Get("Ardents-Expected-Principal"); got != "p_a" {
		t.Fatalf("expected principal = %q", got)
	}
	if got := captured.Header.Get("Ardents-Scopes"); got != "node.status,diagnostics.snapshot" {
		t.Fatalf("scopes = %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
