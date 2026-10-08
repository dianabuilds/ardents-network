//go:build installation_native

package installation

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"testing"
)

func TestInstallationNativePredecessorCloseRefusesPendingJoin(t *testing.T) {
	observation := &process.Invocation{}
	original := &installedPredecessor{
		scopes:        &cgroup.Lifetime{},
		process:       &installedProcessPin{observation: observation},
		stopRequested: true,
	}
	if err := original.close(); !errors.Is(err, ErrBinding) {
		t.Fatal("pending original join became completed cleanup", err)
	}
	if original.process == nil || original.process.observation != observation || original.scopes == nil {
		t.Fatal("pending original descriptor custody was released")
	}
}
