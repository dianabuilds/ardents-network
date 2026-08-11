package namedsite

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

var errResolutionReplay = errors.New("resolution query replay")

type resolutionQueryGate struct {
	runID     string
	networkID string
	target    string
	now       func() time.Time
	mu        sync.Mutex
	seen      map[string]bool
}

func newResolutionQueryGate(runID, networkID, target string, now func() time.Time) (*resolutionQueryGate, error) {
	if runID == "" || networkID == "" || target == "" || now == nil {
		return nil, errors.New("resolution query gate requires bound identity and clock")
	}
	return &resolutionQueryGate{runID: runID, networkID: networkID, target: target, now: now, seen: make(map[string]bool)}, nil
}

func (gate *resolutionQueryGate) accept(reader io.Reader) (resolutionQuery, []byte, error) {
	padded, err := io.ReadAll(io.LimitReader(reader, resolutionMessageSize+1))
	if err != nil {
		return resolutionQuery{}, nil, err
	}
	payload, err := unpadResolutionMessage(padded)
	if err != nil {
		return resolutionQuery{}, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var query resolutionQuery
	if err := decoder.Decode(&query); err != nil {
		return resolutionQuery{}, nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return resolutionQuery{}, nil, errors.New("resolution query has trailing data")
	}
	nonce, err := hex.DecodeString(query.Nonce)
	now := gate.now()
	lookupBound := query.Type == "name" && query.Lookup == "site.reference" || query.Type == "reachability" && query.Lookup == gate.target
	if err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != query.Nonce || query.Schema != resolutionSchema || query.RunID != gate.runID || query.NetworkID != gate.networkID || query.DeadlineUnix <= now.Unix() || query.DeadlineUnix > now.Add(15*time.Second).Unix() || !lookupBound {
		return resolutionQuery{}, nil, errors.New("resolution query binding is invalid")
	}
	gate.mu.Lock()
	replayed := gate.seen[query.Nonce]
	gate.seen[query.Nonce] = true
	gate.mu.Unlock()
	if replayed {
		return resolutionQuery{}, nil, errResolutionReplay
	}
	return query, nonce, nil
}
