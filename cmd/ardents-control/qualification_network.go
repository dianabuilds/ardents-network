package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

func prepareQualificationNetwork(operation string, arguments []string, receipt io.Writer) error {
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var planPath, root string
	flags.StringVar(&planPath, "plan", "", "absolute public initial Network plan")
	flags.StringVar(&root, "output-root", "", "new absolute preparation directory")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || !filepath.IsAbs(planPath) || filepath.Clean(planPath) != planPath || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("initial Network preparation arguments are invalid")
	}
	var message []byte
	var name string
	switch operation {
	case "prepare-qualification-node-record":
		var plan struct {
			Schema string                    `json:"schema"`
			Record epoch.InitialClosedRecord `json:"record"`
		}
		if err := readInitialNetworkPlan(planPath, &plan); err != nil {
			return err
		}
		if plan.Schema != "ardents-qualification-node-record-plan-v1" {
			return errors.New("initial Node Record plan schema is invalid")
		}
		value, err := epoch.PrepareInitialClosedRecord(plan.Record)
		if err != nil {
			return err
		}
		message = value
		name = "node-record.signing-input"
	case "prepare-qualification-epoch":
		var plan struct {
			Schema string                   `json:"schema"`
			Epoch  epoch.InitialClosedEpoch `json:"epoch"`
		}
		if err := readInitialNetworkPlan(planPath, &plan); err != nil {
			return err
		}
		if plan.Schema != "ardents-qualification-epoch-plan-v1" {
			return errors.New("initial Epoch plan schema is invalid")
		}
		value, err := epoch.PrepareInitialClosedEpoch(plan.Epoch)
		if err != nil {
			return err
		}
		message = value
		name = "epoch.unsigned"
	default:
		return errors.New("initial Network preparation operation is invalid")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return err
	}
	output, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
		directory, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
			return err
		}
	}
	digest := sha256.Sum256(message)
	return json.NewEncoder(receipt).Encode(map[string]any{"schema": "ardents-qualification-network-prepared-v1", "operation": operation, "unsigned_sha256": hex.EncodeToString(digest[:]), "signed": false, "state_accepted": false})
}

func readInitialNetworkPlan(path string, plan any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		return errors.Join(errors.New("initial Network plan must be a regular file"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	if len(raw) > 8<<20 {
		return errors.New("initial Network plan exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(plan); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("initial Network plan must contain exactly one JSON object")
	}
	return nil
}
