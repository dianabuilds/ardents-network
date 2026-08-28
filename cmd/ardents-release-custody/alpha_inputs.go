package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

const (
	maximumRequestFileBytes  = 12 << 20
	maximumArtifactFileBytes = 64 << 20
)

func runAssemble(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet("assemble", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, requestPath, endpointPath, controlPath, outputPath string
	flags.StringVar(&root, "root", "", "owner-only release custody directory")
	flags.StringVar(&requestPath, "request", "", "fixed public alpha-input request")
	flags.StringVar(&endpointPath, "endpoint", "", "exact linux-amd64 Endpoint artifact")
	flags.StringVar(&controlPath, "control", "", "exact linux-amd64 control artifact")
	flags.StringVar(&outputPath, "output", "", "previously absent static output directory")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || !allAbsolute(root, requestPath, endpointPath, controlPath, outputPath) {
		return errors.New("release custody assemble arguments are invalid")
	}
	request, err := readBoundedCommandFile(requestPath, maximumRequestFileBytes)
	if err != nil {
		return fmt.Errorf("read alpha-input request: %w", err)
	}
	endpoint, err := readBoundedCommandFile(endpointPath, maximumArtifactFileBytes)
	if err != nil {
		return fmt.Errorf("read alpha Endpoint artifact: %w", err)
	}
	control, err := readBoundedCommandFile(controlPath, maximumArtifactFileBytes)
	if err != nil {
		return fmt.Errorf("read alpha control artifact: %w", err)
	}
	receipt, err := custody.BuildAlphaInputs(ctx, custody.BuildAlphaInputsConfig{Root: root, Request: request,
		Endpoint: endpoint, Control: control, Output: outputPath}, input)
	if err != nil {
		return err
	}
	return renderAlphaInputsReceipt(output, receipt)
}

func runAssembleSuccessor(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet("assemble-successor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, requestPath, endpointPath, controlPath, predecessorPath, outputPath string
	flags.StringVar(&root, "root", "", "owner-only release custody directory")
	flags.StringVar(&requestPath, "request", "", "fixed public RC2 request")
	flags.StringVar(&endpointPath, "endpoint", "", "exact linux-amd64 RC2 Endpoint artifact")
	flags.StringVar(&controlPath, "control", "", "exact linux-amd64 RC2 control artifact")
	flags.StringVar(&predecessorPath, "predecessor", "", "direct RC1 static input directory")
	flags.StringVar(&outputPath, "output", "", "previously absent RC2 static output directory")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || !allAbsolute(root, requestPath, endpointPath, controlPath, predecessorPath, outputPath) {
		return errors.New("release custody assemble-successor arguments are invalid")
	}
	request, err := readBoundedCommandFile(requestPath, maximumRequestFileBytes)
	if err != nil {
		return fmt.Errorf("read RC2 request: %w", err)
	}
	endpoint, err := readBoundedCommandFile(endpointPath, maximumArtifactFileBytes)
	if err != nil {
		return fmt.Errorf("read RC2 Endpoint artifact: %w", err)
	}
	control, err := readBoundedCommandFile(controlPath, maximumArtifactFileBytes)
	if err != nil {
		return fmt.Errorf("read RC2 control artifact: %w", err)
	}
	receipt, err := custody.BuildAlphaSuccessor(ctx, custody.BuildAlphaSuccessorConfig{Root: root, Request: request, Endpoint: endpoint,
		Control: control, Predecessor: predecessorPath, Output: outputPath}, input)
	if err != nil {
		return err
	}
	return renderAlphaInputsReceipt(output, receipt)
}

func allAbsolute(values ...string) bool {
	for _, value := range values {
		if value == "" || !filepath.IsAbs(value) {
			return false
		}
	}
	return true
}

func readBoundedCommandFile(path string, maximum int64) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("input is not a bounded direct regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) || openedInfo.Size() < 1 || openedInfo.Size() > maximum {
		return nil, errors.New("input is not a bounded direct regular file")
	}
	value, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	finalInfo, err := file.Stat()
	if err != nil || !os.SameFile(openedInfo, finalInfo) || len(value) == 0 || int64(len(value)) > maximum || finalInfo.Size() != int64(len(value)) {
		return nil, errors.New("input changed or exceeded its bound while being read")
	}
	return value, nil
}

func renderAlphaInputsReceipt(output io.Writer, receipt custody.AlphaInputsReceipt) error {
	files := make([]struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	}, len(receipt.Files))
	for index, file := range receipt.Files {
		files[index].Name, files[index].Size, files[index].SHA256 = file.Name, file.Size, hex.EncodeToString(file.Digest[:])
	}
	return json.NewEncoder(output).Encode(struct {
		Schema         string `json:"schema"`
		EnvelopeSHA256 string `json:"envelope_sha256"`
		RequestSHA256  string `json:"request_sha256"`
		EndpointSHA256 string `json:"endpoint_sha256"`
		ControlSHA256  string `json:"control_sha256"`
		OutputSHA256   string `json:"output_sha256"`
		Cohort         string `json:"cohort"`
		Release        string `json:"release"`
		SourceRevision string `json:"source_revision"`
		NotBeforeUnix  int64  `json:"not_before_unix"`
		NotAfterUnix   int64  `json:"not_after_unix"`
		TUFVersion     uint64 `json:"tuf_version"`
		CatalogVersion uint64 `json:"catalog_version"`
		Preflight      string `json:"preflight"`
		Files          any    `json:"files"`
	}{Schema: "ardents-alpha-inputs-receipt-v1", EnvelopeSHA256: hex.EncodeToString(receipt.EnvelopeDigest[:]),
		RequestSHA256: hex.EncodeToString(receipt.RequestDigest[:]), EndpointSHA256: hex.EncodeToString(receipt.EndpointDigest[:]),
		ControlSHA256: hex.EncodeToString(receipt.ControlDigest[:]), OutputSHA256: hex.EncodeToString(receipt.OutputDigest[:]),
		Cohort: receipt.Cohort, Release: receipt.Release, SourceRevision: receipt.SourceRevision,
		NotBeforeUnix: receipt.NotBeforeUnix, NotAfterUnix: receipt.NotAfterUnix, TUFVersion: receipt.TUFVersion,
		CatalogVersion: receipt.CatalogVersion, Preflight: receipt.Preflight, Files: files})
}
