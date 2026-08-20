package nameresolution

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"time"
)

// Execute performs one fixed-size private control exchange without a fallback.
func (client *controlClient) Execute(ctx context.Context, raw []byte,
	at time.Time,
) (controlResult, error) {
	if !client.begin() || ctx == nil || at.IsZero() || at.UnixNano() != client.plan.SelectionAt {
		return controlResult{}, errors.New("private naming control input is invalid")
	}
	operation, err := decodeControlOperation(raw)
	if err != nil {
		return controlResult{}, err
	}
	digest, err := controlDigest(operation)
	challenge := client.plan.AdmissionChallenge
	if err != nil || digest != challenge.OperationDigest {
		return controlResult{}, errors.New("private naming control admission does not bind the operation")
	}
	operation.Network = client.plan.NetworkID
	operation.Deadline = client.plan.Deadline
	if _, err := rand.Read(operation.Nonce[:]); err != nil {
		return controlResult{}, err
	}
	proof, _ := challenge.Solve()
	payload, err := controlRequest(operation, proof)
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
	if err != nil || response.Network != operation.Network || response.Nonce != operation.Nonce ||
		response.Deadline != operation.Deadline || response.Kind != operation.Kind || response.Name != operation.Name {
		return controlResult{}, errors.New("private naming control response binding is invalid")
	}
	if response.Result.Class != "accepted" {
		return response.Result, errors.New("private naming control was denied")
	}
	return response.Result, nil
}

func decodeControlOperation(raw []byte) (controlOperation, error) {
	if len(raw) == 0 || len(raw) > 16<<10 {
		return controlOperation{}, errors.New("private naming control operation size is invalid")
	}
	var operation controlOperation
	if err := decodeControlJSON(raw, &operation); err != nil || operation.Network != [32]byte{} ||
		operation.Nonce != [32]byte{} || operation.Deadline != 0 {
		return controlOperation{}, errors.New("private naming control operation is invalid")
	}
	if _, err := controlDigest(operation); err != nil {
		return controlOperation{}, err
	}
	return operation, nil
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
