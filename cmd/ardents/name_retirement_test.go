package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestNameNetworkCommandsRetireBeforeEffects(t *testing.T) {
	fixture := prepareRetiredNameInputs(t)
	stateBefore := retiredNameTree(t, fixture.stateRoot)
	namespaceBefore := retiredNameTree(t, fixture.namespaceRoot)

	var attempts atomic.Int64
	transport := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		attempts.Add(1)
		return nil, errors.New("unexpected Name network attempt")
	}}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		http.DefaultTransport = previousTransport
	})

	requests := [][]string{
		{"resolve", fixture.resolvePath, "alice", fixture.isolation},
		{"control", fixture.controlPath, fixture.operationPath, fixture.isolation},
		{"resolve", filepath.Join(t.TempDir(), "missing-input.json"), "alice", fixture.isolation},
		{"control", filepath.Join(t.TempDir(), "missing-input.json"), filepath.Join(t.TempDir(), "missing-operation.json"), fixture.isolation},
		{"resolve"},
		{"control"},
	}
	for _, request := range requests {
		var output bytes.Buffer
		err := run(t.Context(), append([]string{"name"}, request...), &output)
		if err == nil || err.Error() != "name network command is retired; protected Service Name access is not selected" {
			t.Fatalf("retired %s error = %v", request[0], err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired %s output = %q", request[0], output.String())
		}
	}
	if got := attempts.Load(); got != 0 {
		t.Fatalf("retired Name network attempts = %d", got)
	}
	retiredNameTreeUnchanged(t, fixture.stateRoot, stateBefore)
	retiredNameTreeUnchanged(t, fixture.namespaceRoot, namespaceBefore)
}
