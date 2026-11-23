package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
)

// unit emits bytes only. initialize-trust is a separate explicit first-pin
// action. check is the mandatory privileged ExecStartPre: retained Release only,
// no installation/start/stop/recovery effect and no accepting receipt to replay.
func runPlatformNative(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) < 2 {
		return 2
	}
	if args[0] == "stage-endpoint" && len(args) == 2 {
		bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return runPlatformStageEndpoint(bounded, args[1], out)
	}
	if args[0] == "stage-endpoint" && len(args) == 3 {
		bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return runPlatformStageEndpointSuccessor(bounded, args[1], args[2], out)
	}
	initial := args[0] == "initialize-trust"
	if (args[0] != "unit" && args[0] != "check" && !initial) || (args[0] == "unit" && len(args) != 2) || (args[0] != "unit" && len(args) != 3) {
		return 2
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	files := &artifacts.PlatformInputs{}
	err := executePlatformCheck(bounded, args, out, files)
	err = errors.Join(err, files.Close(), bounded.Err())
	if err != nil {
		return 1
	}
	return 0
}

func executePlatformCheck(ctx context.Context, args []string, out io.Writer, files *artifacts.PlatformInputs) error {
	declaration, err := launch.ReadDeclaration(files, args[1])
	if err != nil {
		return err
	}
	if declaration.Role() == "endpoint" {
		return executeEndpointPlatformCheck(ctx, args, out, files, declaration.Bytes())
	}
	operation := launch.RenderUnit
	var pin string
	switch args[0] {
	case "check":
		operation, pin = launch.CheckStart, args[2]
	case "initialize-trust":
		operation, pin = launch.InitializeTrust, args[2]
	}
	return declaration.Verify(ctx, operation, pin, func(unit []byte) error {
		_, err := out.Write(unit)
		return err
	})
}
