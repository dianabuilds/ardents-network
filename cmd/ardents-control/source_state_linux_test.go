package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func TestSourcePreparationConfigHasNoServingOrAcquisition(t *testing.T) {
	plan := source.ServingDeclaration{ProcessProfile: processresource.SourceProfile, ParticipationRoot: "/var/lib/source-participation",
		Network: &source.ServingNetwork{Root: "/var/lib/source-state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}},
			Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: "/var/lib/source-clock"},
		Address: "127.0.0.1:4000", Certificate: "/unopened/certificate", PrivateKey: "/unopened/private-key", ClientRoot: "/unopened/ca", ClientPins: [][32]byte{{3}}}
	config, err := sourcePreparationStateConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	if config.Root != plan.Network.Root || config.Clock == nil || !config.Now.IsZero() || config.ClockObservationFile != plan.Network.ClockObservationFile || config.LocalRoleStateRoot != "" ||
		config.Source.ServeAddress != "" || config.Source.ServeCertificate.PrivateKey != nil || config.Source.ClientCertificate.PrivateKey != nil {
		t.Fatal("offline preparation acquired runtime configuration")
	}
	if _, details, err := source.New(config.Source, config.Authorities); err != nil || details.Configured || details.Serving {
		t.Fatal("offline configuration declares Source transport", details, err)
	}
	// Declaration projection supplies no accepted State or trusted time. State's
	// genuine verifier separately checks the pins, signatures, time and floors.
	for _, root := range []string{plan.Network.Root, plan.Network.Root + "/child", "/var/lib"} {
		changed := plan
		network := *plan.Network
		changed.Network = &network
		changed.ParticipationRoot = root
		if _, err := sourcePreparationStateConfig(changed); err == nil {
			t.Fatal("overlapping roots accepted", root)
		}
	}
}

func TestSourcePreparationInputsRetainActualFilesAndLatchLoss(t *testing.T) {
	for _, change := range []string{"replace", "same-inode", "cancel"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			path := filepath.Join(t.TempDir(), "public")
			original := []byte("original-public-input\n")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			inputs := &sourceControlInputs{ctx: ctx}
			defer inputs.close()
			raw, err := inputs.read(path, 128)
			if err != nil || !bytes.Equal(raw, original) {
				t.Fatal(string(raw), err)
			}
			if err := inputs.check(); err != nil {
				t.Fatal("own read changed original custody", err)
			}
			switch change {
			case "replace":
				if err := os.Rename(path, path+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
			case "same-inode":
				if err := os.WriteFile(path, []byte("changed-original-input\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				cancel()
			}
			failure := inputs.check()
			if failure == nil || change == "cancel" && !errors.Is(failure, context.Canceled) {
				t.Fatal("original loss not retained", failure)
			}
			if change == "replace" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+".retained", path); err != nil {
					t.Fatal(err)
				}
			} else if change == "same-inode" {
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if inputs.check() != failure {
				t.Fatal("restoration replaced original failure")
			}
		})
	}
}

func TestSourcePreparationRefusesOtherPrincipalBeforeInputEffects(t *testing.T) {
	if os.Geteuid() >= 100 && os.Geteuid() <= 999 {
		t.Fatal("ordinary test profile requires an unprivileged development user outside the fixed system-account range")
	}
	var out bytes.Buffer
	if err := runSourceControl([]string{"prepare-state", "/unopened-plan", "/unopened-bundle"}, &out); err == nil || out.Len() != 0 {
		t.Fatal("foreign principal reached preparation or receipt", err)
	}
}

func TestSourcePreparationReobservesEarlierNamespaceAfterLaterRead(t *testing.T) {
	for _, change := range []string{"unchanged", "file", "ancestor"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			earlier := filepath.Join(root, "earlier")
			if err := os.Mkdir(earlier, 0700); err != nil {
				t.Fatal(err)
			}
			first, last := filepath.Join(earlier, "public"), filepath.Join(root, "last")
			for _, path := range []string{first, last} {
				if err := os.WriteFile(path, []byte("same public bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			inputs := &sourceControlInputs{ctx: t.Context()}
			defer inputs.close()
			for _, path := range []string{first, last} {
				if _, err := inputs.read(path, 128); err != nil {
					t.Fatal(err)
				}
			}
			lastFile := inputs.files[len(inputs.files)-1].file
			entered, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- inputs.observeRead(func(file *os.File, raw []byte) (int, error) {
					if file == lastFile {
						close(entered)
						<-release
					}
					return file.ReadAt(raw, 0)
				})
			}()
			<-entered // All earlier native content and link checks have finished.
			var mutation error
			if change == "file" {
				mutation = os.Rename(first, first+".original")
				if mutation == nil {
					mutation = os.WriteFile(first, []byte("same public bytes"), 0600)
				}
			} else if change == "ancestor" {
				mutation = os.Rename(earlier, earlier+".original")
				if mutation == nil {
					mutation = os.Mkdir(earlier, 0700)
				}
				if mutation == nil {
					mutation = os.WriteFile(first, []byte("same public bytes"), 0600)
				}
			}
			close(release)
			err := <-done // Join the actual final native read before releasing inputs.
			if mutation != nil || (err == nil) != (change == "unchanged") {
				t.Fatalf("post-read namespace observation: change=%s mutation=%v result=%v", change, mutation, err)
			}
		})
	}
}
