//go:build linux

package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

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

func readPlatformAccount(files *artifacts.PlatformInputs, name string) (uint32, error) {
	passwd, group, err := files.ReadAccountFiles()
	if err != nil {
		return 0, err
	}
	uid, ok := process.DedicatedAccountID(passwd, group, name)
	if !ok {
		return 0, errPlatformInput
	}
	return uid, nil
}
