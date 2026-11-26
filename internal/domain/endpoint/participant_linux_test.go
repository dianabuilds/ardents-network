//go:build linux

package endpoint

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/execution"
)

func TestCanceledParticipantDoesNotReadStartupOrReportProgress(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	phase, err := Run(ctx, root, func(Progress) error { t.Fatal("canceled participant emitted progress"); return nil })
	if phase != "installed-startup" || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUnavailable) {
		t.Fatal(phase, err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("startup root changed", err)
	}
}

func TestReaderDeclarationDoesNotAcquirePublisherGrant(t *testing.T) {
	encode := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	plan := HeadlessPlan{Role: "reader", BrokerID: encode(1), ConnectionPrincipal: encode(2), AdministrationPrincipal: "not-an-identity"}
	config, err := installedLocalConfig(plan)
	if err != nil || len(config.Grants) != 1 || config.Grants[0].Surface != execution.Connection {
		t.Fatal(config, err)
	}
	plan.Role = ""
	if _, err := installedLocalConfig(plan); err == nil {
		t.Fatal("dual-role declaration ignored invalid Publisher identity")
	}
}

func TestSourceInputRefusesOriginalPreparationLossBeforeRead(t *testing.T) {
	root := t.TempDir()
	encoded := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	plan := HeadlessPlan{NetworkStateRoot: filepath.Join(root, "state"), TimeConfidenceFile: filepath.Join(root, "clock"), NetworkID: encoded(1), ClosedProfileAuthority: encoded(2), NetworkAuthorities: []string{encoded(3)}, NetworkThreshold: 1}
	source := SourcePlan{ClockObservedAt: time.Now().UTC().Format(time.RFC3339), OrderSeed: encoded(4), ClientCertificate: filepath.Join(root, "missing-certificate")}
	source.Sources = make([]struct {
		Address        string `json:"address"`
		ServerName     string `json:"server_name"`
		Identity       string `json:"identity"`
		Family         string `json:"family"`
		EndpointHandle string `json:"endpoint_handle"`
		RootCA         string `json:"root_ca"`
		LeafKeyDigest  string `json:"leaf_key_digest"`
	}, 2)
	lost := errors.New("original preparation lost")
	_, err := NetworkConfig(plan, source, func() error { return lost })
	if !errors.Is(err, lost) || errors.Is(err, os.ErrNotExist) {
		t.Fatal("read preceded original check", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("configuration touched runtime roots", err)
	}
}
