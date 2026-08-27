package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

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
