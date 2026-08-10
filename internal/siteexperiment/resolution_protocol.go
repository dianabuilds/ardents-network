package siteexperiment

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

const resolutionSchema = "gatec-resolution/v1"

type resolutionQuery struct {
	Schema       string `json:"schema"`
	Type         string `json:"type"`
	Lookup       string `json:"lookup"`
	Nonce        string `json:"nonce"`
	RunID        string `json:"run_id"`
	NetworkID    string `json:"network_id"`
	DeadlineUnix int64  `json:"deadline_unix"`
}

type resolutionGateway struct {
	fixture *authorityFixture
	mu      sync.Mutex
	seen    map[string]bool
	now     func() time.Time
}

func newResolutionGateway(fixture *authorityFixture, now func() time.Time) (*resolutionGateway, error) {
	if fixture == nil || now == nil {
		return nil, errors.New("resolution Gateway fixture and clock are required")
	}
	return &resolutionGateway{fixture: fixture, seen: make(map[string]bool), now: now}, nil
}

func (gateway *resolutionGateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	padded, err := io.ReadAll(io.LimitReader(request.Body, resolutionMessageSize+1))
	if err != nil || len(padded) != resolutionMessageSize {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	payload, err := unpadResolutionMessage(padded)
	if err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var query resolutionQuery
	if err := decoder.Decode(&query); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	nonce, err := hex.DecodeString(query.Nonce)
	now := gateway.now()
	if err != nil || len(nonce) != 32 || query.Schema != resolutionSchema || query.RunID != gateway.fixture.runID || query.NetworkID != gateway.fixture.networkID || query.DeadlineUnix <= now.Unix() || query.DeadlineUnix > now.Add(15*time.Second).Unix() {
		http.Error(writer, "invalid binding", http.StatusBadRequest)
		return
	}
	gateway.mu.Lock()
	replayed := gateway.seen[query.Nonce]
	gateway.seen[query.Nonce] = true
	gateway.mu.Unlock()
	if replayed {
		http.Error(writer, "replay", http.StatusConflict)
		return
	}
	var response []byte
	switch {
	case query.Type == "name" && query.Lookup == "site.reference":
		response, err = gateway.fixture.signedNameRecord(nonce, time.Unix(query.DeadlineUnix, 0))
	case query.Type == "reachability" && query.Lookup == gateway.fixture.target:
		response, err = gateway.fixture.signedDescriptor(nonce, time.Unix(query.DeadlineUnix, 0))
	default:
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(writer, "fixture failure", http.StatusInternalServerError)
		return
	}
	fixed, err := padResolutionMessage(response)
	if err != nil {
		http.Error(writer, "fixture overflow", http.StatusInternalServerError)
		return
	}
	_, _ = writer.Write(fixed)
}

func makeResolutionQuery(kind, lookup, runID, networkID string, now time.Time) ([]byte, []byte, error) {
	if kind != "name" && kind != "reachability" || lookup == "" || runID == "" || networkID == "" {
		return nil, nil, errors.New("resolution query is outside the fixed contract")
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	query := resolutionQuery{Schema: resolutionSchema, Type: kind, Lookup: lookup, Nonce: hex.EncodeToString(nonce), RunID: runID, NetworkID: networkID, DeadlineUnix: now.Add(15 * time.Second).Unix()}
	payload, err := json.Marshal(query)
	if err != nil {
		return nil, nil, err
	}
	fixed, err := padResolutionMessage(payload)
	return fixed, nonce, err
}

func resolveMessage(ctx context.Context, transport http.RoundTripper, kind, lookup, runID, networkID string, now time.Time) ([]byte, []byte, error) {
	query, nonce, err := makeResolutionQuery(kind, lookup, runID, networkID, now)
	if err != nil {
		return nil, nil, err
	}
	response, err := sendOHTTPMessage(ctx, transport, query)
	if err != nil {
		return nil, nil, err
	}
	payload, err := unpadResolutionMessage(response)
	return payload, nonce, err
}
