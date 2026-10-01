//go:build ignore

// Command prepare-qualification-alpha-catalog writes one unsigned initial
// catalog signing input. It creates no keys, signatures or acceptance rights.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func main() {
	if err := prepareCatalog(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepareCatalog() error {
	if len(os.Args) != 3 || !filepath.IsAbs(os.Args[1]) || filepath.Clean(os.Args[1]) != os.Args[1] || !filepath.IsAbs(os.Args[2]) || filepath.Clean(os.Args[2]) != os.Args[2] {
		return errors.New("requires absolute public catalog plan and new absolute output directory")
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		return errors.Join(errors.New("catalog plan must be a regular file"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return errors.New("catalog plan exceeds its bound")
	}
	var plan struct {
		Schema  string               `json:"schema"`
		Catalog alphacontrol.Catalog `json:"catalog"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("catalog plan must contain exactly one JSON object")
	}
	if plan.Schema != "ardents-qualification-alpha-catalog-plan-v1" {
		return errors.New("catalog plan schema is invalid")
	}
	message, err := alphacontrol.PrepareInitialCatalogSigningInput(plan.Catalog)
	if err != nil {
		return err
	}
	root := os.Args[2]
	if err := os.Mkdir(root, 0700); err != nil {
		return err
	}
	output, err := os.OpenFile(filepath.Join(root, "catalog.signing-input"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, writeErr := output.Write(message)
	if writeErr == nil && n != len(message) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, output.Sync(), output.Close()); err != nil {
		return err
	}
	for _, path := range []string{root, filepath.Dir(root)} {
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := errors.Join(dir.Sync(), dir.Close()); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "ardents-qualification-alpha-catalog-prepared-v1", "signed": false, "authenticated": false})
}
