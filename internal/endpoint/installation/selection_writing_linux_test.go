//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitialSelectionRetainsManagerReloadFailure(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	journal, err := beginGenerationJournal(root, selected)
	if err != nil {
		t.Fatal(err)
	}
	body, err := canonicalJSON(selected)
	if err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("original manager reload refused")
	calls := 0
	// This invokes only the post-admission transition, not installed admission.
	err = publishInitialSelection(context.Background(), root, selected, body, checkedBinding{}, journal, func(_ context.Context, arguments ...string) (string, error) {
		calls++
		if len(arguments) != 1 || arguments[0] != "daemon-reload" {
			t.Fatal("unexpected manager operation", arguments)
		}
		if current, err := os.ReadFile(filepath.Join(root, "selection.json")); err != nil || !bytes.Equal(current, body) {
			t.Fatal("manager reload preceded complete selection", err)
		}
		return "", refusal
	})
	if !errors.Is(err, refusal) || calls != 1 {
		t.Fatal("reload refusal lost or retried", err, calls)
	}
	failure, err := readInstalledFile(filepath.Join(journal, "selection-failure.json"), 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var record transitionRecord
	if err := decodeCanonical(failure, 64<<10, &record); err != nil || record.Phase != "selection-failed" || record.OriginalError != refusal.Error() || record.GenerationDigest != selected.GenerationDigest || record.BindingDigest != selected.BindingDigest {
		t.Fatal("original reload failure not bound to selection", err)
	}
	if _, err := os.Lstat(filepath.Join(journal, "0007.json")); !os.IsNotExist(err) {
		t.Fatal("reload refusal claimed installed-stopped", err)
	}
}

func TestStoppedUnitRefusesActiveOrForeignManagerProperties(t *testing.T) {
	name := "ardents-endpoint.service"
	body := "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/" + name + "\nDropInPaths=\nMainPID=0\n"
	if err := verifyStoppedUnit(body, name); err != nil {
		t.Fatal(err)
	}
	for label, changed := range map[string]string{
		"active":           strings.Replace(body, "ActiveState=inactive", "ActiveState=active", 1),
		"process":          strings.Replace(body, "MainPID=0", "MainPID=42", 1),
		"foreign fragment": strings.Replace(body, "/etc/systemd/system/", "/run/systemd/system/", 1),
		"drop-in":          strings.Replace(body, "DropInPaths=", "DropInPaths=/etc/systemd/system/override.conf", 1),
		"incomplete":       strings.Replace(body, "MainPID=0\n", "", 1),
		"duplicate":        body + "MainPID=0\n",
	} {
		t.Run(label, func(t *testing.T) {
			if err := verifyStoppedUnit(changed, name); err == nil {
				t.Fatal("unsafe manager observation accepted")
			}
		})
	}
}
