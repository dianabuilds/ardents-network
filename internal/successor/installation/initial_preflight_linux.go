package installation

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// These are the accepted fixed destinations. Their absence never authorizes
// adoption of a later foreign inode or grants platform/Execution admission.
func fixedResourceNames() map[string]string {
	return map[string]string{
		"/usr/lib/ardents/text-worker-root/ardents-text":      "ardents-text-linux-amd64",
		"/etc/systemd/system/ardents-text-reader@.service":    "ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service": "ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket":      "ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket":   "ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules":   "50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf":               "ardents-text.conf",
		"/etc/systemd/system/ardents-endpoint.service":        "ardents-endpoint.service",
	}
}

// Preflight is an observation inside native preparation, not a reusable proof.
// Platform/protection admission must precede it in the actual provision owner.
// Repeat these observations under that operation's lease before native effects.
func preflightInitialEffects(ctx context.Context, request Request) error {
	if ctx == nil || request.declared == nil || request.ManifestSHA256() == "" {
		return ErrInput
	}
	if err := observeRequestCustody(ctx, request); err != nil {
		return err
	}
	_, err := user.Lookup("ardents-endpoint")
	var missingUser user.UnknownUserError
	if err == nil || !errors.As(err, &missingUser) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	_, err = user.LookupGroup("ardents-endpoint")
	var missingGroup user.UnknownGroupError
	if err == nil || !errors.As(err, &missingGroup) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	paths, err := writableDirectories(*request.declared)
	if err != nil {
		return err
	}
	paths = append(paths, request.declared.InstallationRoot, "/usr/lib/ardents/text-worker-root",
		"/etc/ardents/text-worker-artifact.json", "/run/ardents-text", request.declared.Headless.ApplicationSocket)
	if request.declared.Headless.Role == "" {
		paths = append(paths, request.declared.Headless.AdministrationSocket)
	}
	for filename := range fixedResourceNames() {
		paths = append(paths, filename)
	}
	for _, filename := range paths {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	return observeRequestCustody(ctx, request)
}

func observeRequestCustody(ctx context.Context, request Request) error {
	if ctx == nil || request.custody == nil || request.declared == nil {
		return ErrInput
	}
	fresh, err := ReadOwnedRequest(ctx, request.custody.path, request.ManifestSHA256() != "")
	if err != nil {
		return err
	}
	wanted, err := canonicalJSON(*request.declared)
	if err != nil || fresh.custody == nil || fresh.custody.digest != request.custody.digest ||
		sha256.Sum256(wanted) != request.custody.digest ||
		!sameRequestFile(request.custody.identity, fresh.custody.identity) {
		return errors.Join(ErrBinding, err)
	}
	return ctx.Err()
}

func requireAbsentManagedPath(filename string) error {
	if !canonicalPath(filename) || filename == "/" {
		return ErrInput
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	for parent := filepath.Dir(filename); ; parent = filepath.Dir(parent) {
		_, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = rootDirectoryAncestors(parent)
		return err
	}
}

type boundedManagerOutput struct{ body strings.Builder }

func (output *boundedManagerOutput) Write(body []byte) (int, error) {
	if output.body.Len()+len(body) > 64<<10 {
		return 0, ErrNativeUnavailable
	}
	return output.body.Write(body)
}

func observeAbsentInstallationUnits(ctx context.Context) error {
	if ctx == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/systemctl", "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-endpoint.service", "ardents-text-*")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		return errors.Join(ErrNativeUnavailable, err, bounded.Err(), ctx.Err())
	}
	if strings.TrimSpace(output.body.String()) != "" || diagnostic.body.Len() != 0 {
		return ErrNativeUnavailable
	}
	return errors.Join(bounded.Err(), ctx.Err())
}
