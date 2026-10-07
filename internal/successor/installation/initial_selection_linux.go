package installation

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (owned *initialPreparation) publishInitialSelection(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	stage := owned.stage
	if err := stage.fixedPhase(ctx, "0005.json", "publishing-selection"); err != nil {
		return err
	}
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		return err
	}
	if err := stage.createFixedFile(ctx, filepath.Join(stage.lease.path, "selection.json"), body, 0640, owned.prepared.gid); err != nil {
		return err
	}
	if err := stage.fixedPhase(ctx, "0006.json", "reloading-manager"); err != nil {
		return err
	}
	if _, err := installationManager(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := owned.observeStoppedInstallation(ctx, request); err != nil {
		return err
	}
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	return stage.fixedPhase(ctx, "0007.json", "installed-stopped")
}

func observeStoppedManager(ctx context.Context) error {
	for _, unit := range []string{"ardents-endpoint.service", "ardents-text-reader.socket", "ardents-text-publisher.socket"} {
		properties := "--property=LoadState,ActiveState,SubState,FragmentPath,DropInPaths"
		if unit == "ardents-endpoint.service" {
			properties += ",MainPID"
		}
		body, err := installationManager(ctx, "--no-pager", "show", properties, unit)
		if err != nil {
			return err
		}
		if err := verifyStoppedManagerUnit(body, unit); err != nil {
			return err
		}
	}
	body, err := installationManager(ctx, "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-text-reader@*.service", "ardents-text-publisher@*.service")
	if err != nil || strings.TrimSpace(body) != "" {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return ctx.Err()
}

func verifyStoppedManagerUnit(body, unit string) error {
	expected := map[string]string{"LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/" + unit, "DropInPaths": ""}
	if unit == "ardents-endpoint.service" {
		expected["MainPID"] = "0"
	} else if unit != "ardents-text-reader.socket" && unit != "ardents-text-publisher.socket" {
		return ErrBinding
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		wanted, known := expected[key]
		if !found || !known || seen[key] || value != wanted {
			return ErrBinding
		}
		seen[key] = true
	}
	if len(seen) != len(expected) {
		return ErrBinding
	}
	return nil
}

// These fixed calls are transaction mechanisms; outputs are observations,
// never substitute authorizations. Original caller cancellation remains final.
func installationManager(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/systemctl", args...)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := errors.Join(command.Run(), bounded.Err(), ctx.Err()); err != nil {
		return "", err
	}
	if diagnostic.body.Len() != 0 {
		return "", ErrNativeUnavailable
	}
	return output.body.String(), nil
}
