package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	nameresolution "github.com/dianabuilds/ardents-network/internal/naming/resolution"
)

type controlReceipt struct {
	Schema string `json:"schema"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Class  string `json:"class"`
}

func runControl(inputPath, operationPath string, isolation [32]byte, output io.Writer,
	transport *http.Transport, load resolutionViewLoader) error {
	input, config, selection, err := readNetworkInput(inputPath, controlInputSchema)
	if err != nil || selection.ConnectionRendezvousNodeID != [32]byte{} {
		return errors.New("private naming control input is invalid")
	}
	view, err := load(config)
	if err != nil {
		return errors.New("authenticated Network State is unavailable")
	}
	client, err := nameresolution.OpenControl(view, selection, input.GatewayProfile, isolation, transport)
	if err != nil {
		return err
	}
	raw, err := readOperatorInput(operationPath, 16<<10)
	if err != nil {
		return err
	}
	var identity struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	if err = json.Unmarshal(raw, &identity); err != nil {
		return err
	}
	result, err := client.Execute(context.Background(), raw, selection.At)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(controlReceipt{Schema: "ardents-name-control-result-v1", Kind: identity.Kind,
		Name: identity.Name, Class: result.Class})
}
