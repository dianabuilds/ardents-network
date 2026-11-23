package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/publication/instance"
)

func runPublication(ctx context.Context, args []string, out io.Writer) int {
	if len(args) == 0 {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	switch args[0] {
	case "prepare-root":
		return preparePublicationRoot(ctx, args, out)
	case "instance-initialize":
		if len(args) != 5 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		network, err := admissionHex(args[2], 32)
		if err != nil {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		before, err := admissionTime(args[3])
		if err != nil {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		after, err := admissionTime(args[4])
		if err != nil {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		request, err := instance.InitializeRequest(ctx, instance.Config{
			Root: args[1], NetworkID: [32]byte(network), NotBefore: before, NotAfter: after,
		})
		return instanceRequestResult(out, request, err)
	case "instance-request":
		if len(args) != 2 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		request, err := instance.ReadRequest(ctx, args[1])
		return instanceRequestResult(out, request, err)
	case "instance-accept":
		if len(args) != 3 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		raw, err := readBounded(args[2], 1024)
		if err != nil {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		accepted, err := instance.AcceptResponse(ctx, args[1], raw)
		if err != nil {
			return enrollmentReport(out, "instance-unavailable", 1)
		}
		if json.NewEncoder(out).Encode(struct {
			Outcome    string              `json:"outcome"`
			Acceptance instance.Acceptance `json:"acceptance"`
		}{"instance-response-accepted", accepted}) != nil {
			return 2
		}
		return 0
	default:
		return enrollmentReport(out, "publication-invalid", 2)
	}
}

func preparePublicationRoot(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 4 {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	target, err := admissionHex(args[2], 32)
	if err != nil {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	network, err := admissionHex(args[3], 32)
	if err != nil {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	floor, err := durable.PrepareRoot(ctx, durable.Config{
		Root: args[1], Target: [32]byte(target), Network: [32]byte(network),
	})
	if err != nil {
		return enrollmentReport(out, "publication-unavailable", 1)
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
		Floor   uint64 `json:"floor"`
	}{"publication-root-prepared", floor}) != nil {
		return 2
	}
	return 0
}

func instanceRequestResult(out io.Writer, request []byte, requestErr error) int {
	if requestErr != nil {
		return enrollmentReport(out, "instance-unavailable", 1)
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
		Request []byte `json:"request"`
	}{"instance-request-prepared", request}) != nil {
		return 2
	}
	return 0
}
