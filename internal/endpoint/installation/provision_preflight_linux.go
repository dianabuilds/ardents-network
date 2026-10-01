//go:build linux

package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func preflightInitialInstallation(ctx context.Context, request Request) error {
	_, err := user.Lookup("ardents-endpoint")
	var missingUser user.UnknownUserError
	if err == nil || !errors.As(err, &missingUser) {
		return errors.Join(errors.New("initial Endpoint account is existing or unavailable"), err)
	}
	_, err = user.LookupGroup("ardents-endpoint")
	var missingGroup user.UnknownGroupError
	if err == nil || !errors.As(err, &missingGroup) {
		return errors.Join(errors.New("initial Endpoint group is existing or unavailable"), err)
	}
	paths := append([]string{request.InstallationRoot, "/usr/lib/ardents/text-worker-root", "/etc/ardents/text-worker-artifact.json", "/run/ardents-text"}, writableDirectories(request)...)
	for path := range fixedResources() {
		paths = append(paths, path)
	}
	paths = append(paths, request.Headless.ApplicationSocket)
	if request.Headless.Role == "" {
		paths = append(paths, request.Headless.AdministrationSocket)
	}
	for _, path := range paths {
		if err := requireAbsentManagedPath(path); err != nil {
			return err
		}
	}
	return refuseLoadedInstallationUnits(ctx)
}

func refuseLoadedInstallationUnits(ctx context.Context) error {
	output, err := runInstallationManager(ctx, "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-endpoint.service", "ardents-text-*")
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "" {
		return errors.New("initial installation has existing loaded Endpoint or text worker units")
	}
	return ctx.Err()
}

func requireAbsentManagedPath(path string) error {
	if !canonicalPath(path) {
		return errors.New("initial installation managed path is invalid")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("initial installation managed path exists or is unavailable: %s", path)
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		_, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		return checkRootAncestors(parent, false)
	}
}

type commandOutput struct{ body strings.Builder }

func (output *commandOutput) Write(body []byte) (int, error) {
	if output.body.Len()+len(body) > 64<<10 {
		return 0, errors.New("installation manager output exceeds its bound")
	}
	return output.body.Write(body)
}

// Only fixed manager operations are supplied by installation code, never shell
// fragments or command names from the root request.
func runInstallationManager(ctx context.Context, arguments ...string) (string, error) {
	return runInstallationCommand(ctx, "/usr/bin/systemctl", arguments...)
}

func runInstallationCommand(ctx context.Context, program string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, program, arguments...)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	var output, diagnostic commandOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("installation manager operation: %w; %s", err, diagnostic.body.String())
	}
	return output.body.String(), nil
}
