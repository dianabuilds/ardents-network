//go:build linux

package installation

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func admitInstalledStart(ctx context.Context, root string) (runtimeplan.DecodedHeadless, error) {
	refuse := func(err error) (runtimeplan.DecodedHeadless, error) { return runtimeplan.DecodedHeadless{}, err }
	if os.Geteuid() == 0 {
		return refuse(errors.New("installed Endpoint cannot run as root"))
	}
	checked, err := readLocalBinding(root, readInstalledFile)
	if err != nil {
		return refuse(err)
	}
	if uint32(os.Geteuid()) != checked.binding.UID || uint32(os.Getegid()) != checked.binding.GID {
		return refuse(errors.New("installed Endpoint process account differs"))
	}
	groups, err := os.Getgroups()
	if err != nil {
		return refuse(err)
	}
	for _, group := range groups {
		if uint32(group) != checked.binding.GID {
			return refuse(errors.New("installed Endpoint has foreign supplementary groups"))
		}
	}
	if err := observeBinding(checked); err != nil {
		return refuse(err)
	}
	program := filepath.Join(checked.directory, "ardents-linux-amd64")
	if !slices.Equal(os.Args, []string{program, "endpoint", "start-installed", root}) {
		return refuse(errors.New("installed Endpoint process arguments differ"))
	}
	executable, err := os.Executable()
	if err != nil || executable != program {
		return refuse(errors.New("installed Endpoint executable path differs"))
	}
	actual, err := os.Stat("/proc/self/exe")
	if err != nil {
		return refuse(err)
	}
	expected, err := os.Stat(program)
	if err != nil || !os.SameFile(actual, expected) {
		return refuse(errors.New("installed Endpoint executable identity differs"))
	}
	invocationBytes, err := hex.DecodeString(os.Getenv("INVOCATION_ID"))
	var invocation [16]byte
	if err != nil || len(invocationBytes) != 16 || hex.EncodeToString(invocationBytes) != os.Getenv("INVOCATION_ID") {
		return refuse(errors.New("installed Endpoint invocation is unavailable"))
	}
	copy(invocation[:], invocationBytes)
	if invocation == [16]byte{} {
		return refuse(errors.New("installed Endpoint invocation is zero"))
	}
	cgroup, err := readProcessFile("/proc/self/cgroup", 4096)
	if err != nil || string(cgroup) != "0::/system.slice/ardents-endpoint.service\n" {
		return refuse(errors.New("installed Endpoint cgroup differs"))
	}
	if err := worker.VerifyEndpointService(ctx); err != nil {
		return refuse(err)
	}
	unit, service, err := worker.ReadEndpointProperties(ctx)
	if err != nil {
		return refuse(err)
	}
	if err := verifyInstalledProcess(unit, service, checked, uint32(os.Getpid()), invocation); err != nil {
		return refuse(err)
	}
	if err := observeInstalledSockets(ctx, unit); err != nil {
		return refuse(err)
	}
	// Type=exec lets the root transition observe this real main process before
	// participant composition. It keeps its cursor until that observation is
	// durable; a crashed transition never opens this finite accepting barrier.
	selected := selection{GenerationDigest: checked.binding.GenerationDigest, BindingDigest: digestHex(checked.files["binding.json"])}
	if err := awaitStartCompletion(ctx, root, selected, invocation); err != nil {
		return refuse(err)
	}
	current, err := readLocalBinding(root, readInstalledFile)
	if err != nil || current.binding.GenerationDigest != checked.binding.GenerationDigest ||
		digestHex(current.files["binding.json"]) != digestHex(checked.files["binding.json"]) {
		return refuse(errors.New("installed selection changed during start admission"))
	}
	unit, service, err = worker.ReadEndpointProperties(ctx)
	if err != nil {
		return refuse(err)
	}
	if err := verifyInstalledProcess(unit, service, current, uint32(os.Getpid()), invocation); err != nil {
		return refuse(err)
	}
	if err := observeInstalledSockets(ctx, unit); err != nil {
		return refuse(err)
	}
	if err := ctx.Err(); err != nil {
		return refuse(err)
	}
	return runtimeplan.DecodeHeadless(checked.files["headless.json"])
}

func refusePendingTransition(root string) error {
	for _, name := range []string{"transition.json", "transition-failure.json", "start-guard.json"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			return errors.New("installed Endpoint requires explicit transition recovery")
		}
	}
	return nil
}

func readProcessFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if int64(len(body)) > maximum {
		return nil, errors.Join(errors.New("installed process observation exceeds its bound"), readErr, closeErr)
	}
	return body, errors.Join(readErr, closeErr)
}
