package command

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source/inspection"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSourceInspection(parent context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) != 10 {
		fmt.Fprintln(diagnostic, "usage: ardents-next source inspect-latest SERVE_PLAN CLIENT_CERT CLIENT_KEY SERVER_CA SERVER_NAME SERVER_KEY_PIN SOURCE_ID FAMILY HANDLE MATERIAL_INDEX")
		return 2
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	request, err := sourceInspectionRequest(args)
	if err != nil {
		fmt.Fprintln(diagnostic, "source inspection refused")
		return 1
	}
	response, err := inspection.InspectLatest(ctx, request)
	if err != nil {
		fmt.Fprintln(diagnostic, "source inspection refused")
		return 1
	}
	raw, err := json.Marshal(struct {
		Operation    string   `json:"operation"`
		Status       string   `json:"status"`
		ObjectDigest [32]byte `json:"object_digest"`
		Payload      []byte   `json:"payload"`
	}{"source.inspect-latest", response.Status, response.ObjectDigest, response.Payload})
	if err != nil || artifacts.WriteReceipt(ctx, out, append(raw, '\n')) != nil {
		return 1
	}
	return 0
}

func sourceInspectionRequest(args []string) (inspection.Request, error) {
	request := inspection.Request{
		ServingPlan: args[0], ClientCertificate: args[1], ClientKey: args[2], ServerCA: args[3],
		ServerName: args[4], Family: args[7], Handle: args[8],
	}
	for i, destination := range []*[32]byte{&request.ServerKeyPin, &request.SourceID} {
		raw, err := hex.DecodeString(args[5+i])
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != args[5+i] {
			return inspection.Request{}, artifacts.ErrInput
		}
		copy(destination[:], raw)
	}
	index, err := strconv.ParseUint(args[9], 10, 32)
	if err != nil || index >= 64 || strconv.FormatUint(index, 10) != args[9] {
		return inspection.Request{}, artifacts.ErrInput
	}
	request.MaterialIndex = uint32(index)
	return request, nil
}
