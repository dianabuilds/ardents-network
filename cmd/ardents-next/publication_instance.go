package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
)

func runInstance(ctx context.Context, args []string, out io.Writer) int {
	if len(args) == 0 {
		return enrollmentReport(out, "instance-invalid", 2)
	}
	switch args[0] {
	case "instance-initialize":
		if len(args) != 5 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		networkRaw, err := admissionHex(args[2], 32)
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
		var network [32]byte
		copy(network[:], networkRaw)
		root, err := instance.Initialize(ctx, instance.Config{Root: args[1], NetworkID: network, NotBefore: before, NotAfter: after})
		if err != nil {
			return enrollmentReport(out, "instance-unavailable", 1)
		}
		return instanceRequestResult(ctx, root, out)
	case "instance-request":
		if len(args) != 2 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		root, err := instance.Open(ctx, args[1])
		if err != nil {
			return enrollmentReport(out, "instance-unavailable", 1)
		}
		return instanceRequestResult(ctx, root, out)
	case "instance-accept":
		if len(args) != 3 {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		raw, err := readBounded(args[2], 1024)
		if err != nil {
			return enrollmentReport(out, "instance-invalid", 2)
		}
		root, err := instance.Open(ctx, args[1])
		if err != nil {
			return enrollmentReport(out, "instance-unavailable", 1)
		}
		result, acceptErr := root.Accept(ctx, raw)
		if acceptErr == nil {
			credential, err := root.Credential(ctx)
			if err != nil || credential.Delegation().Generation != result.Generation || credential.Digest() != result.CredentialDigest {
				acceptErr = instance.ErrUnavailable
			}
		}
		closeErr := root.Close()
		if acceptErr != nil || closeErr != nil || ctx.Err() != nil {
			return enrollmentReport(out, "instance-unavailable", 1)
		}
		if json.NewEncoder(out).Encode(struct {
			Outcome    string              `json:"outcome"`
			Acceptance instance.Acceptance `json:"acceptance"`
		}{"instance-response-accepted", result}) != nil {
			return 2
		}
		return 0
	default:
		return enrollmentReport(out, "instance-invalid", 2)
	}
}

func instanceRequestResult(ctx context.Context, root *instance.Root, out io.Writer) int {
	raw, requestErr := root.Request(ctx)
	closeErr := root.Close()
	if requestErr != nil || closeErr != nil || ctx.Err() != nil {
		return enrollmentReport(out, "instance-unavailable", 1)
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
		Request []byte `json:"request"`
	}{"instance-request-prepared", raw}) != nil {
		return 2
	}
	return 0
}
