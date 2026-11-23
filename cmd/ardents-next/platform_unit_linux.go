//go:build linux

package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

const platformUnitTemplate = systemd.PlatformUnitTemplate

func platformUser(p platformLaunch) string {
	if p.Role == "endpoint" {
		return "ardents-endpoint"
	}
	return systemd.InfrastructureAccount(p.Role)
}

func infrastructurePlatformUnit(p platformLaunch, manifest, pin string) systemd.InfrastructureUnit {
	return systemd.InfrastructureUnit{
		Role: p.Role, Distribution: p.Distribution, Program: p.Program, Configuration: p.Configuration,
		Manifest: manifest, ManifestSHA256: pin, WritableRoots: p.WritableRoots,
	}
}

func renderPlatformUnit(p platformLaunch, manifest, pin string) []byte {
	return infrastructurePlatformUnit(p, manifest, pin).Render()
}

func observePlatformPrestart(ctx context.Context, p platformLaunch, manifest, pin string) error {
	return infrastructurePlatformUnit(p, manifest, pin).ObservePrestart(ctx)
}
