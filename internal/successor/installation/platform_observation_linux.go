package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Preserve current native refusal while #359's separate capability repair is
// unresolved. This is not another accepting version policy or qualification.
func observeInstallationPlatform(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if os.Geteuid() != 0 || runtime.GOARCH != "amd64" {
		return ErrNativeUnavailable
	}
	release, err := observePlatformRelease(ctx)
	if err != nil {
		return err
	}
	version, err := observeSystemManagerVersion(ctx)
	if err != nil {
		return err
	}
	if !(release == "22.04" && (version == "249" || strings.HasPrefix(version, "249.")) || release == "24.04" && (version == "255" || strings.HasPrefix(version, "255."))) {
		return ErrNativeUnavailable
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs("/sys/fs/cgroup", &filesystem); err != nil || filesystem.Type != 0x63677270 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return ctx.Err()
}

func observePlatformRelease(ctx context.Context) (result string, returnedErr error) {
	ancestors, err := rootDirectoryAncestors("/usr/lib")
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot("/usr/lib")
	if err != nil {
		return "", err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	before, err := root.Lstat("os-release")
	if err != nil || !ownedRequestFile(before) || before.Size() <= 0 || before.Size() > 16<<10 {
		return "", errors.Join(ErrNativeUnavailable, err)
	}
	file, err := root.OpenFile("os-release", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !sameRequestFile(before, info) {
		return "", errors.Join(ErrNativeUnavailable, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, (16<<10)+1))
	if err != nil || int64(len(body)) != before.Size() {
		return "", errors.Join(ErrNativeUnavailable, err)
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if _, duplicate := fields[key]; !found || duplicate {
			return "", ErrNativeUnavailable
		}
		fields[key] = strings.Trim(value, "\"")
	}
	final, err := file.Stat()
	pathFinal, pathErr := root.Lstat("os-release")
	if err != nil || pathErr != nil || !sameRequestFile(before, final) || !sameRequestFile(before, pathFinal) || fields["ID"] != "ubuntu" {
		return "", errors.Join(ErrNativeUnavailable, err, pathErr)
	}
	for name, original := range ancestors {
		current, err := os.Lstat(name)
		if err != nil || !rootDirectory(current) || !os.SameFile(original, current) {
			return "", errors.Join(ErrNativeUnavailable, err)
		}
	}
	return fields["VERSION_ID"], ctx.Err()
}

func observeSystemManagerVersion(ctx context.Context) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/busctl", "--system", "--json=short", "--no-pager", "get-property", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "Version")
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
	var answer struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}
	body := []byte(strings.TrimSpace(output.body.String()))
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(answer)
	if err != nil || !bytes.Equal(body, canonical) || answer.Type != "s" || answer.Data == "" {
		return "", errors.Join(ErrNativeUnavailable, err)
	}
	return answer.Data, ctx.Err()
}
