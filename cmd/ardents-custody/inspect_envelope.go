package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func inspectEnvelope(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-envelope", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var path string
	flags.StringVar(&path, "envelope", "", "canonical custody envelope")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || path == "" {
		return errors.New("inspect-envelope requires envelope")
	}
	info, err := custody.InspectEnvelope(path)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema         string `json:"schema"`
		Operation      string `json:"operation"`
		Purpose        string `json:"purpose"`
		CiphertextSize uint64 `json:"ciphertext_size"`
		Digest         string `json:"digest"`
	}{
		Schema: "ardents-custody-inspection-v1", Operation: "inspect-envelope", Purpose: string(info.Purpose), CiphertextSize: info.CiphertextSize, Digest: hex.EncodeToString(info.Digest[:]),
	})
}
