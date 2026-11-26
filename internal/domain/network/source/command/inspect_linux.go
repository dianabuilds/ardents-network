package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/inspection"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSourceInspection(parent context.Context, args []string, out, diagnostic io.Writer) int {
	if parent == nil || len(args) != 10 {
		return writeInspectionDiagnostic(diagnostic, 2, "usage: ardents-next source inspect-latest SERVE_PLAN CLIENT_CERT CLIENT_KEY SERVER_CA SERVER_NAME SERVER_KEY_PIN SOURCE_ID FAMILY HANDLE MATERIAL_INDEX")
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	request, err := sourceInspectionRequest(args)
	if err != nil {
		return writeInspectionDiagnostic(diagnostic, 1, "source inspection refused")
	}
	response, err := inspection.InspectLatest(ctx, request)
	if err != nil {
		return writeInspectionDiagnostic(diagnostic, 1, "source inspection refused")
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

func writeInspectionDiagnostic(diagnostic io.Writer, code int, message string) int {
	if _, err := fmt.Fprintln(diagnostic, message); err != nil {
		return 2
	}
	return code
}

func sourceInspectionRequest(args []string) (inspection.Request, error) {
	request := inspection.Request{
		ServingPlan: args[0], ClientCertificate: args[1], ClientKey: args[2], ServerCA: args[3],
		ServerName: args[4], Family: args[7], Handle: args[8],
	}
	for i, destination := range []*[32]byte{&request.ServerKeyPin, &request.SourceID} {
		value, ok := canonicaldigest.Decode(args[5+i])
		if !ok {
			return inspection.Request{}, artifacts.ErrInput
		}
		copy(destination[:], value[:])
	}
	index, err := strconv.ParseUint(args[9], 10, 32)
	if err != nil || index >= 64 || strconv.FormatUint(index, 10) != args[9] {
		return inspection.Request{}, artifacts.ErrInput
	}
	request.MaterialIndex = uint32(index)
	return request, nil
}
