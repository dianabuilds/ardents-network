package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
)

type evidencePlan struct {
	Schema        string                           `json:"schema"`
	NotBefore     time.Time                        `json:"not_before"`
	NotAfter      time.Time                        `json:"not_after"`
	Release       inspection.ReleaseEvidence       `json:"release"`
	Network       inspection.NetworkEvidence       `json:"network"`
	Compatibility inspection.CompatibilityEvidence `json:"compatibility"`
}

func prepareEvidence(arguments []string, receipt io.Writer) error {
	flags := flag.NewFlagSet("prepare-qualification-evidence", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var planPath, root string
	flags.StringVar(&planPath, "plan", "", "absolute public qualification plan")
	flags.StringVar(&root, "output-root", "", "new absolute preparation directory")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("qualification preparation arguments are invalid")
	}
	if !filepath.IsAbs(planPath) || filepath.Clean(planPath) != planPath || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("requires public evidence plan and new absolute output directory")
	}
	file, err := os.Open(planPath)
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		return errors.Join(errors.New("evidence plan must be a regular file"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (32<<20)+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if len(raw) > 32<<20 {
		return errors.New("evidence plan exceeds its bound")
	}
	var plan evidencePlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("evidence plan must contain exactly one JSON object")
	}
	if plan.Schema != "ardents-qualification-alpha-evidence-plan-v1" {
		return errors.New("evidence plan schema is invalid")
	}
	values, err := inspection.PrepareInitialEvidence(plan.Release, plan.Network, plan.Compatibility)
	if err != nil {
		return err
	}
	var signingInputs [3][]byte
	for index, value := range values {
		signingInputs[index], err = alphacontrol.PrepareInitialComponentSigningInput(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1, NotBefore: plan.NotBefore, NotAfter: plan.NotAfter, Body: value})
		if err != nil {
			return err
		}
	}

	if err := os.Mkdir(root, 0700); err != nil {
		return err
	}
	for index, name := range []string{"release.payload", "network.payload", "compatibility.payload"} {
		output, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		n, writeErr := output.Write(values[index])
		if writeErr == nil && n != len(values[index]) {
			writeErr = io.ErrShortWrite
		}
		if err := errors.Join(writeErr, output.Sync(), output.Close()); err != nil {
			return err
		}
		request, err := os.OpenFile(filepath.Join(root, name+".signing-input"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		n, writeErr = request.Write(signingInputs[index])
		if writeErr == nil && n != len(signingInputs[index]) {
			writeErr = io.ErrShortWrite
		}
		if err := errors.Join(writeErr, request.Sync(), request.Close()); err != nil {
			return err
		}
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
	return json.NewEncoder(receipt).Encode(map[string]any{"schema": "ardents-qualification-alpha-evidence-prepared-v1", "signed": false, "authenticated": false})
}
