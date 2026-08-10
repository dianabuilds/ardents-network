package siteexperiment

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestOHTTPAdapterSeparatesRoleViewsAndRejectsModification(t *testing.T) {
	t.Parallel()
	marker := []byte("ardents://site.reference|target-test-marker")
	plaintext, err := padResolutionMessage(marker)
	if err != nil {
		t.Fatal(err)
	}

	var lock sync.Mutex
	var relayBody, gatewayBody []byte
	var relayOrigin, gatewayOrigin string
	keyConfig, gateway, err := newOHTTPGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
			return
		}
		lock.Lock()
		gatewayBody = append([]byte(nil), body...)
		lock.Unlock()
		response, padErr := padResolutionMessage([]byte("signed-gatec-fixture-response"))
		if padErr != nil {
			t.Error(padErr)
			return
		}
		_, _ = w.Write(response)
	}))
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		gatewayOrigin = r.RemoteAddr
		lock.Unlock()
		gateway.ServeHTTP(w, r)
	}))
	t.Cleanup(gatewayServer.Close)

	relay := testOHTTPRelay(t, gatewayServer.URL, false, func(body []byte, origin string) {
		lock.Lock()
		relayBody = append([]byte(nil), body...)
		relayOrigin = origin
		lock.Unlock()
	})
	t.Cleanup(relay.Close)
	transport, err := newOHTTPTransport(keyConfig, relay.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	response, err := sendOHTTPMessage(t.Context(), transport, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != resolutionMessageSize {
		t.Fatalf("response length = %d", len(response))
	}

	lock.Lock()
	observedRelayBody := append([]byte(nil), relayBody...)
	observedGatewayBody := append([]byte(nil), gatewayBody...)
	observedRelayOrigin, observedGatewayOrigin := relayOrigin, gatewayOrigin
	lock.Unlock()
	if bytes.Contains(observedRelayBody, marker) {
		t.Fatal("Relay observed exact Name/Target marker")
	}
	if !bytes.Equal(observedGatewayBody, plaintext) {
		t.Fatal("Gateway did not receive the exact fixed-size plaintext")
	}
	if observedRelayOrigin == "" || observedGatewayOrigin == "" || observedRelayOrigin == observedGatewayOrigin {
		t.Fatalf("origins not separated: Relay=%q Gateway=%q", observedRelayOrigin, observedGatewayOrigin)
	}

	tamperingRelay := testOHTTPRelay(t, gatewayServer.URL, true, nil)
	t.Cleanup(tamperingRelay.Close)
	tamperedTransport, err := newOHTTPTransport(keyConfig, tamperingRelay.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sendOHTTPMessage(t.Context(), tamperedTransport, plaintext); err == nil {
		t.Fatal("modified OHTTP response was accepted")
	}
}

func testOHTTPRelay(t *testing.T, gatewayURL string, tamper bool, observe func([]byte, string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if observe != nil {
			observe(body, r.RemoteAddr)
		}
		forward, err := http.NewRequestWithContext(r.Context(), http.MethodPost, gatewayURL, bytes.NewReader(body))
		if err != nil {
			t.Error(err)
			return
		}
		forward.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		response, err := http.DefaultClient.Do(forward)
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if tamper && len(responseBody) > 0 {
			responseBody[len(responseBody)-1] ^= 1
		}
		w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(responseBody)
	}))
}
