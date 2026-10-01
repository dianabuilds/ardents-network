//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func selectInitialInstallation(ctx context.Context, root string, selected selection) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return errors.New("installation selection requires root and context")
	}
	body, err := canonicalJSON(selected)
	if err != nil {
		return err
	}
	checked, err := readLocalBinding(root, func(path string, maximum int64) ([]byte, error) {
		if path == filepath.Join(root, "selection.json") {
			return body, nil
		}
		return readInstalledFile(path, maximum)
	})
	if err != nil {
		return err
	}
	if err := observeAccountAndRoots(checked, false); err != nil {
		return err
	}
	if err := observeFixedResources(checked); err != nil {
		return err
	}
	if err := refuseLoadedInstallationUnits(ctx); err != nil {
		return err
	}
	lease, err := acquireInstallationLease(root)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, lease.Close()) }()
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := requireJournalPhases(journal, selected, []string{"writing-generation", "generation-staged", "installing-fixed-resources", "fixed-resources-installed"}); err != nil {
		return err
	}
	return publishInitialSelection(ctx, root, selected, body, checked, journal, runInstallationManager)
}

// The caller retains admission checks and the writer lease throughout this
// transition. The manager seam isolates a refusal after durable selection;
// production always supplies the concrete installation manager.
func publishInitialSelection(ctx context.Context, root string, selected selection, body []byte, checked checkedBinding, journal string, manager func(context.Context, ...string) (string, error)) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := appendGenerationRecord(journal, "0005.json", selected, "publishing-selection", nil); err != nil {
		return err
	}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, appendGenerationRecord(journal, "selection-failure.json", selected, "selection-failed", returnedErr))
		}
	}()
	if err := createInstallationFile(ctx, filepath.Join(root, "selection.json"), body, 0640, checked.binding.GID, journal, selected); err != nil {
		return err
	}
	if err := syncDirectory(root); err != nil {
		return err
	}
	if err := appendGenerationRecord(journal, "0006.json", selected, "reloading-manager", nil); err != nil {
		return err
	}
	if _, err := manager(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := observeStoppedInstallation(ctx); err != nil {
		return err
	}
	if err := observeBinding(checked); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return appendGenerationRecord(journal, "0007.json", selected, "installed-stopped", nil)
}

func observeStoppedInstallation(ctx context.Context) error {
	for _, name := range []string{"ardents-endpoint.service", "ardents-text-reader.socket", "ardents-text-publisher.socket"} {
		properties := "--property=LoadState,ActiveState,SubState,FragmentPath,DropInPaths"
		if name == "ardents-endpoint.service" {
			properties += ",MainPID"
		}
		body, err := runInstallationManager(ctx, "--no-pager", "show", properties, name)
		if err != nil {
			return err
		}
		if err := verifyStoppedUnit(body, name); err != nil {
			return err
		}
	}
	workers, err := runInstallationManager(ctx, "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-text-reader@*.service", "ardents-text-publisher@*.service")
	if err != nil {
		return err
	}
	if strings.TrimSpace(workers) != "" {
		return errors.New("stopped installation has loaded worker instances")
	}
	return ctx.Err()
}

func verifyStoppedUnit(body, name string) error {
	expected := map[string]string{"LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/" + name, "DropInPaths": ""}
	if name == "ardents-endpoint.service" {
		expected["MainPID"] = "0"
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		wanted, known := expected[key]
		if !ok || !known || seen[key] || value != wanted {
			return errors.New("installed unit is not exactly loaded and stopped")
		}
		seen[key] = true
	}
	if len(seen) != len(expected) {
		return errors.New("stopped unit properties are incomplete")
	}
	return nil
}
