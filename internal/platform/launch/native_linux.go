//go:build linux

package launch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	endpointlaunch "github.com/dianabuilds/ardents-network/internal/endpoint/launch"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/release"
	infrastructurelaunch "github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
)

func RunNative(ctx context.Context, args []string, out io.Writer) int {
	if !systemd.PrivilegedCommandAvailable(ctx) || len(args) < 2 {
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
	declaration, err := infrastructurelaunch.ReadDeclaration(files, args[1])
	if err != nil {
		return err
	}
	if declaration.Role() == "endpoint" {
		return executeEndpointPlatformCheck(ctx, args, out, files, declaration.Bytes())
	}
	operation := infrastructurelaunch.RenderUnit
	var pin string
	switch args[0] {
	case "check":
		operation, pin = infrastructurelaunch.CheckStart, args[2]
	case "initialize-trust":
		operation, pin = infrastructurelaunch.InitializeTrust, args[2]
	}
	return declaration.Verify(ctx, operation, pin, func(unit []byte) error {
		_, err := out.Write(unit)
		return err
	})
}

func executeEndpointPlatformCheck(ctx context.Context, args []string, out io.Writer, files *artifacts.PlatformInputs, raw []byte) error {
	operation := endpointlaunch.PlatformRenderUnit
	var pin string
	switch args[0] {
	case "check":
		operation, pin = endpointlaunch.PlatformCheckStart, args[2]
	case "initialize-trust":
		operation, pin = endpointlaunch.PlatformInitializeTrust, args[2]
	}
	unit, err := endpointlaunch.VerifyPlatform(ctx, files, operation, args[1], pin, raw)
	if err != nil || operation != endpointlaunch.PlatformRenderUnit {
		return err
	}
	_, err = out.Write(unit)
	return err
}

func runPlatformStageEndpoint(ctx context.Context, filename string, out io.Writer) int {
	err := endpointlaunch.StagePlatformInitial(ctx, filename)
	if errors.Is(err, endpointlaunch.ErrPlatformIncompatible) {
		return writeDeliveryReport(out, "installation-incompatible", 1)
	}
	if err != nil {
		return reportSoftwareDeliveryFailure(ctx, out, err)
	}
	return writeDeliveryReport(out, "generation-staged", 0)
}

func runPlatformStageEndpointSuccessor(ctx context.Context, filename, previousManifest string, out io.Writer) int {
	if err := endpointlaunch.StagePlatformReplacement(ctx, filename, previousManifest); err != nil {
		return reportSoftwareDeliveryFailure(ctx, out, err)
	}
	return writeDeliveryReport(out, "generation-staged", 0)
}

func reportSoftwareDeliveryFailure(ctx context.Context, out io.Writer, err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return writeDeliveryReport(out, "installation-canceled", 130)
	}
	if errors.Is(err, release.ErrTrustUnavailable) {
		return writeDeliveryReport(out, "installation-trust-unavailable", 1)
	}
	return writeDeliveryReport(out, "installation-refused", 1)
}

func writeDeliveryReport(out io.Writer, outcome string, code int) int {
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
	}{outcome}) != nil {
		return 2
	}
	return code
}
