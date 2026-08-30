package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

type serviceInstanceInitializationPlan struct {
	Schema    string `json:"schema"`
	Root      string `json:"root"`
	NetworkID string `json:"network_id"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
}

func runServiceInstance(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 3 || arguments[0] != "initialize" || arguments[1] != "--config" {
		return errors.New("usage: ardents service-instance initialize --config PATH")
	}
	var plan serviceInstanceInitializationPlan
	if err := decodeOperatorInput(arguments[2], 8<<10, &plan); err != nil {
		return err
	}
	if plan.Schema != "ardents-service-instance-initialize-v1" || !filepath.IsAbs(plan.Root) ||
		filepath.Clean(plan.Root) != plan.Root {
		return errors.New("Service Instance initialization plan is not canonical")
	}
	config := instance.InitializeConfig{Root: plan.Root}
	if err := decodeOperatorFixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return err
	}
	var err error
	config.NotBefore, err = parseCanonicalServiceTime(plan.NotBefore)
	if err != nil {
		return err
	}
	config.NotAfter, err = parseCanonicalServiceTime(plan.NotAfter)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	root, err := instance.Initialize(config)
	if err != nil {
		return err
	}
	request, requestErr := root.Request()
	closeErr := root.Close()
	if requestErr != nil {
		return errors.Join(requestErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	digest := sha256.Sum256(request)
	return json.NewEncoder(output).Encode(struct {
		Schema        string `json:"schema"`
		Request       []byte `json:"request"`
		RequestSHA256 string `json:"request_sha256"`
	}{Schema: "ardents-service-instance-request-v1", Request: request,
		RequestSHA256: hex.EncodeToString(digest[:])})
}

func parseCanonicalServiceTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.Format(time.RFC3339) != value || !parsed.Equal(parsed.UTC().Truncate(time.Second)) {
		return time.Time{}, errors.New("Service Instance time is invalid")
	}
	return parsed.UTC(), nil
}
