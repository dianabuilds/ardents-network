//go:build referencec2

package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	reference "github.com/dianabuilds/ardents-network/internal/browserreference"
)

func TestExerciseDynamicReferenceKeepsOneProxyConnectionAcrossNoFallbackProbes(t *testing.T) {
	bridgeSide, applicationSide := net.Pipe()
	t.Cleanup(func() { _ = applicationSide.Close() })
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target:     [32]byte{1},
		Hostname:   "reference.ard",
		Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	route, err := proxy.RegisterTransparent("reference.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = route.Close() })
	client, err := alphaReferenceClient("http://reference.ard/", proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client.Timeout = time.Second
	t.Cleanup(client.CloseIdleConnections)

	proofPath := filepath.Join(t.TempDir(), "dynamic-proof")
	served := make(chan error, 1)
	go func() { served <- serveDynamic(applicationSide, proofPath) }()
	if err := exerciseDynamicReference(client, "http://reference.ard/"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("dynamic Publisher application did not finish")
	}
	proof, err := os.ReadFile(proofPath)
	if err != nil || string(proof) != "dynamic-http\n" {
		t.Fatalf("dynamic Publisher proof = %q / %v", proof, err)
	}
	accepted, rejected := alphaReferenceProxyDialCounts(client)
	if accepted != 1 || rejected != 0 {
		t.Fatalf("proxy TCP dial counts = accepted %d / rejected %d, want 1 / 0", accepted, rejected)
	}
}

func TestAlphaReferenceClientUsesOnlyItsExactLoopbackProxyRoute(t *testing.T) {
	origin, err := reference.Open(reference.Config{Target: [32]byte{1}, Document: reference.Resource{
		ContentType: "text/html", Body: []byte("fixture")}})
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.Register("reference.ard", origin)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()
	client, err := alphaReferenceClient("http://reference.ard/", proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get("http://reference.ard/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "fixture" {
		t.Fatalf("Reference response = %d %q %v", response.StatusCode, body, readErr)
	}
	response, err = client.Get("http://other.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("fixture Reference client sent another name with status %d", response.StatusCode)
	}
}

func TestAlphaReferenceClientRejectsEveryProxyRedial(t *testing.T) {
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	client, err := alphaReferenceClient("http://reference.ard/", proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get("http://reference.ard/")
	if err != nil || response == nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("first proxy route probe = %+v / %v", response, err)
	}
	_ = response.Body.Close()
	client.CloseIdleConnections()
	response, err = client.Get("http://reference.ard/")
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "refused proxy reconnect") {
		t.Fatalf("second proxy dial = %v, want fail-closed reconnect refusal", err)
	}
	accepted, rejected := alphaReferenceProxyDialCounts(client)
	if accepted != 1 || rejected != 1 {
		t.Fatalf("proxy TCP dial counts = accepted %d / rejected %d, want 1 / 1", accepted, rejected)
	}
}
