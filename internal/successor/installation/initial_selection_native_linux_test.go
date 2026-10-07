//go:build installation_native

package installation

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallationNativeSelectionRequiresPromotionAndExactOwnedBytes(t *testing.T) {
	stage := accessStage(t)
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(stage.lease.path, "selection.json")
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.createFixedFile(t.Context(), filename, body, 0640, 65534); !errors.Is(err, ErrBinding) {
		t.Fatal("selection without original phase accepted", err)
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		t.Fatal("pre-intent selection appeared")
	}
	if err := stage.fixedPhase(t.Context(), "0005.json", "publishing-selection"); err != nil {
		t.Fatal(err)
	}
	if err := stage.createFixedFile(t.Context(), filename, []byte("foreign selection\n"), 0640, 65534); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign selection accepted", err)
	}
	if err := stage.createFixedFile(t.Context(), filename, body, 0640, 65534); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("selected bytes differ", err)
	}
	if err := stage.fixedPhase(t.Context(), "0006.json", "reloading-manager"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "0007.json")); !os.IsNotExist(err) {
		t.Fatal("selection fabricated installed completion")
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationNativeStoppedManagerPropertyGrammar(t *testing.T) {
	const endpoint = "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/ardents-endpoint.service\nDropInPaths=\nMainPID=0\n"
	if err := verifyStoppedManagerUnit(endpoint, "ardents-endpoint.service"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		strings.Replace(endpoint, "MainPID=0", "MainPID=123", 1),
		strings.Replace(endpoint, "ActiveState=inactive", "ActiveState=active", 1),
		strings.Replace(endpoint, "DropInPaths=", "DropInPaths=/etc/systemd/system/foreign.conf", 1),
		strings.Replace(endpoint, "FragmentPath=/etc", "FragmentPath=/foreign", 1),
		strings.Replace(endpoint, "MainPID=0\n", "", 1),
		endpoint + "MainPID=0\n", endpoint + "Foreign=accepted\n",
	} {
		if err := verifyStoppedManagerUnit(body, "ardents-endpoint.service"); !errors.Is(err, ErrBinding) {
			t.Fatal("invalid stopped properties accepted", err)
		}
	}
	for _, unit := range []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"} {
		body := "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/" + unit + "\nDropInPaths=\n"
		if err := verifyStoppedManagerUnit(body, unit); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(verifyStoppedManagerUnit(endpoint, "foreign.service"), ErrBinding) {
		t.Fatal("foreign manager unit accepted")
	}
}
