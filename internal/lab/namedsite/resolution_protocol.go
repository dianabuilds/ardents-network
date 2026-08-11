package namedsite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
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
	gate    *resolutionQueryGate
}

func newResolutionGateway(fixture *authorityFixture, now func() time.Time) (*resolutionGateway, error) {
	if fixture == nil || now == nil {
		return nil, errors.New("resolution Gateway fixture and clock are required")
	}
	gate, err := newResolutionQueryGate(fixture.runID, fixture.networkID, fixture.target, now)
	if err != nil {
		return nil, err
	}
	return &resolutionGateway{fixture: fixture, gate: gate}, nil
}

func (gateway *resolutionGateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	query, nonce, err := gateway.gate.accept(request.Body)
	if errors.Is(err, errResolutionReplay) {
		http.Error(writer, "replay", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	var response []byte
	switch query.Type {
	case "name":
		response, err = gateway.fixture.signedNameRecord(nonce, time.Unix(query.DeadlineUnix, 0))
	case "reachability":
		response, err = gateway.fixture.signedDescriptor(nonce, time.Unix(query.DeadlineUnix, 0))
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
