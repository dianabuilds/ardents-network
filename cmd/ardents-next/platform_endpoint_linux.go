//go:build linux

package main

import (
	"context"
	"io"

	endpointlaunch "github.com/dianabuilds/ardents-network/internal/endpoint/launch"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

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
