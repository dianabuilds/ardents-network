package siteexperiment

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"sync"
)

type relayRoleHandler struct {
	mu        sync.Mutex
	requests  int
	cleartext bool
	completed chan struct{}
}

func runResolutionRelayRole(ctx context.Context, evidenceDirectory string) error {
	handler := &relayRoleHandler{completed: make(chan struct{})}
	if err := runResolutionRoleServer(ctx, handler, handler.completed); err != nil {
		return err
	}
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "relay.json"), map[string]any{
		"schema_version": "gatec-resolution-role-view/v1", "status": "completed", "request_count": handler.requests,
		"exact_name_or_target_visible": handler.cleartext, "gateway_origin_visible": true,
	})
}

func (handler *relayRoleHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(io.LimitReader(request.Body, resolutionMessageSize*2))
	if err != nil {
		http.Error(writer, "read", http.StatusBadRequest)
		return
	}
	cleartext := bytes.Contains(body, []byte("site.reference")) || bytes.Contains(body, []byte("target:sha256:"))
	forward, err := http.NewRequestWithContext(request.Context(), http.MethodPost, "http://gateway:8080", bytes.NewReader(body))
	if err != nil {
		http.Error(writer, "request", http.StatusBadGateway)
		return
	}
	forward.Header.Set("Content-Type", request.Header.Get("Content-Type"))
	response, err := http.DefaultClient.Do(forward)
	if err != nil {
		http.Error(writer, "forward", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	writer.WriteHeader(response.StatusCode)
	_, _ = io.Copy(writer, io.LimitReader(response.Body, resolutionMessageSize*2))
	handler.mu.Lock()
	handler.cleartext = handler.cleartext || cleartext
	handler.requests++
	if handler.requests == 2 {
		close(handler.completed)
	}
	handler.mu.Unlock()
}
