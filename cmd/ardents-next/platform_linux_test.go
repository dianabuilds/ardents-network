//go:build linux

package main

import (
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

type platformLaunch = launch.Declaration

func platformDigest(raw []byte) string {
	return endpointfixture.FixtureDigest(raw)
}

const platformUnitTemplate = systemd.PlatformUnitTemplate
