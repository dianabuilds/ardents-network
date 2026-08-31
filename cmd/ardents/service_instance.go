package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

type serviceInstanceInitializationPlan struct {
	Schema      string `json:"schema"`
	Root        string `json:"root"`
	NetworkID   string `json:"network_id"`
	NotBefore   string `json:"not_before"`
	NotAfter    string `json:"not_after"`
	RequestFile string `json:"request_file"`
}

func runServiceInstance(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 && arguments[0] == "accept" {
		return acceptServiceInstance(ctx, arguments[1:], output)
	}
	if len(arguments) != 3 || arguments[0] != "initialize" || arguments[1] != "--config" {
		return errors.New("usage: ardents service-instance initialize --config PATH | accept --root PATH --response PATH")
	}
	var plan serviceInstanceInitializationPlan
	if err := decodeOperatorInput(arguments[2], 8<<10, &plan); err != nil {
		return err
	}
	if plan.Schema != "ardents-service-instance-initialize-v1" || !canonicalAbsolutePath(plan.Root) ||
		!canonicalAbsolutePath(plan.RequestFile) {
		return errors.New("service Instance initialization plan is not canonical")
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
	if err := writeStablePublicFile(plan.RequestFile, request); err != nil {
		return err
	}
	digest := sha256.Sum256(request)
	return json.NewEncoder(output).Encode(struct {
		Schema        string `json:"schema"`
		Request       []byte `json:"request"`
		RequestSHA256 string `json:"request_sha256"`
	}{Schema: "ardents-service-instance-request-v1", Request: request,
		RequestSHA256: hex.EncodeToString(digest[:])})
}

func acceptServiceInstance(ctx context.Context, arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("service-instance accept", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var rootPath, responsePath string
	flags.StringVar(&rootPath, "root", "", "owner-controlled Service Instance root")
	flags.StringVar(&responsePath, "response", "", "canonical public Authority response")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || !canonicalAbsolutePath(rootPath) || !canonicalAbsolutePath(responsePath) {
		return errors.New("service-instance accept requires canonical absolute root and response paths")
	}
	response, err := readOperatorInput(responsePath, 1024)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	root, err := instance.Open(rootPath)
	if err != nil {
		return err
	}
	accepted, acceptErr := root.Accept(response)
	closeErr := root.Close()
	if acceptErr != nil {
		return errors.Join(acceptErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return json.NewEncoder(output).Encode(struct {
		Schema     string         `json:"schema"`
		State      instance.State `json:"state"`
		Generation uint64         `json:"generation"`
	}{Schema: "ardents-service-instance-acceptance-v1", State: accepted.State, Generation: accepted.Generation})
}

func canonicalAbsolutePath(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path
}

func writeStablePublicFile(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readOperatorInput(path, 1024)
		if readErr != nil || string(existing) != string(body) {
			return errors.New("service Instance public request destination conflicts")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func parseCanonicalServiceTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.Format(time.RFC3339) != value || !parsed.Equal(parsed.UTC().Truncate(time.Second)) {
		return time.Time{}, errors.New("service Instance time is invalid")
	}
	return parsed.UTC(), nil
}
