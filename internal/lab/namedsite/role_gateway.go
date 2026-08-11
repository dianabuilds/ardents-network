package namedsite

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type gatewayRoleHandler struct {
	authoritySocket string
	gate            *resolutionQueryGate
	mu              sync.Mutex
	queries         []string
	completed       chan struct{}
}

func runResolutionGatewayRole(ctx context.Context, configPath, keyPath, evidenceDirectory string) error {
	var config clientRoleConfig
	if err := readStrictRoleConfig(configPath, &config); err != nil || config.Schema != roleConfigSchema {
		return errors.New("resolution Gateway role configuration is invalid")
	}
	gate, err := newResolutionQueryGate(config.RunID, config.NetworkID, config.Target, time.Now)
	if err != nil {
		return err
	}
	handler := &gatewayRoleHandler{authoritySocket: "/authority/gateway.sock", gate: gate, completed: make(chan struct{})}
	key, gateway, err := newOHTTPGateway(handler)
	if err != nil {
		return err
	}
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return err
	}
	if keyPath == "" || !filepath.IsAbs(keyPath) || filepath.Base(keyPath) != "key-config.bin" {
		return errors.New("resolution Gateway key path is invalid")
	}
	if err := os.WriteFile(keyPath, keyBytes, 0o644); err != nil {
		return err
	}
	if err := runResolutionRoleServer(ctx, gateway, handler.completed); err != nil {
		return err
	}
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "gateway.json"), map[string]any{
		"schema_version": "gatec-resolution-role-view/v1", "status": "completed", "plaintext_query_types": handler.queries,
		"request_count": len(handler.queries), "authority_private_key_present": false,
	})
}

func (handler *gatewayRoleHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	query, _, err := handler.gate.accept(request.Body)
	if errors.Is(err, errResolutionReplay) {
		http.Error(writer, "replay", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(writer, "invalid lookup", http.StatusBadRequest)
		return
	}
	connection, err := dialRoleSocketRetry(request.Context(), handler.authoritySocket)
	if err != nil {
		http.Error(writer, "authority unavailable", http.StatusBadGateway)
		return
	}
	err = json.NewEncoder(connection).Encode(authorityRequest{Operation: "sign", Type: query.Type, Nonce: query.Nonce, DeadlineUnix: query.DeadlineUnix})
	var response authorityResponse
	if err == nil {
		err = json.NewDecoder(connection).Decode(&response)
	}
	_ = connection.Close()
	if err != nil || len(response.Record) == 0 {
		http.Error(writer, "authority response", http.StatusBadGateway)
		return
	}
	fixed, err := padResolutionMessage(response.Record)
	if err != nil {
		http.Error(writer, "response overflow", http.StatusInternalServerError)
		return
	}
	_, _ = writer.Write(fixed)
	handler.mu.Lock()
	handler.queries = append(handler.queries, query.Type)
	if len(handler.queries) == 2 {
		close(handler.completed)
	}
	handler.mu.Unlock()
}
