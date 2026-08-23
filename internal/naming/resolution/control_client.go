package resolution

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
)

// Execute performs one fixed-size private control exchange without a fallback.
func (client *controlClient) Execute(ctx context.Context, raw []byte,
	at time.Time,
) (controlResult, error) {
	if !client.begin() || ctx == nil || at.IsZero() || at.UnixNano() != client.plan.SelectionAt {
		return controlResult{}, errors.New("private naming control input is invalid")
	}
	submission, err := authority.OpenSubmission(raw)
	if err != nil {
		return controlResult{}, err
	}
	digest := submission.Digest()
	challenge := client.plan.AdmissionChallenge
	if digest != challenge.OperationDigest {
		return controlResult{}, errors.New("private naming control admission does not bind the operation")
	}
	requestBinding := controlBinding{network: client.plan.NetworkID, deadline: client.plan.Deadline}
	if _, err := rand.Read(requestBinding.nonce[:]); err != nil {
		return controlResult{}, err
	}
	proof, _ := challenge.Solve()
	payload, err := controlRequest(submission, requestBinding, proof)
	if err != nil {
		return controlResult{}, err
	}
	attempt, cancel := context.WithDeadline(ctx, time.Unix(0, client.plan.Deadline))
	defer cancel()
	request, err := http.NewRequestWithContext(attempt, http.MethodPost, "http://ohttp.invalid/control", bytes.NewReader(payload))
	if err != nil {
		return controlResult{}, err
	}
	encapsulated, decapsulator, err := client.transport.Encapsulate(request)
	if err != nil {
		return controlResult{}, errors.New("private naming control is unavailable")
	}
	outer, err := client.client.Do(encapsulated)
	client.client.CloseIdleConnections()
	if err != nil {
		return controlResult{}, errors.New("private naming control is unavailable")
	}
	defer outer.Body.Close()
	plain, err := decapsulator.Decapsulate(attempt, outer)
	if err != nil {
		return controlResult{}, errors.New("private naming control evidence is invalid")
	}
	defer plain.Body.Close()
	responseRaw, err := io.ReadAll(io.LimitReader(plain.Body, fixedMessageSize+1))
	if err != nil {
		return controlResult{}, errors.New("private naming control evidence is invalid")
	}
	response, err := decodeControlResponse(responseRaw)
	if err != nil || response.Network != requestBinding.network || response.Nonce != requestBinding.nonce ||
		response.Deadline != requestBinding.deadline || response.OperationDigest != digest {
		return controlResult{}, errors.New("private naming control response binding is invalid")
	}
	if response.Result.Class != "submitted" {
		return response.Result, errors.New("private naming control was denied")
	}
	return response.Result, nil
}

func (client *controlClient) begin() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.used {
		return false
	}
	client.used = true
	return true
}
