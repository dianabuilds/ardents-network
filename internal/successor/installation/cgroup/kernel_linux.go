package cgroup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var errInput = errors.New("invalid installation cgroup observation")

const cgroup2Magic = 0x63677270

// Retain the original kernel events object, not a pathname that can be reused.
// Its owner must finish physical join before closing this descriptor.
type pin struct {
	events   *os.File
	identity os.FileInfo
}

func pinScope(ctx context.Context, unit string) (result *pin, returnedErr error) {
	if ctx == nil || !scopeUnit(unit) {
		return nil, errInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	const parent = "/sys/fs/cgroup/system.slice"
	if err := observeAncestors(parent); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	var directory, events *os.File
	defer func() {
		if directory != nil {
			returnedErr = errors.Join(returnedErr, directory.Close())
		}
		returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err())
		if returnedErr != nil {
			if events != nil {
				returnedErr = errors.Join(returnedErr, events.Close())
			}
			result = nil
		}
	}()
	before, err := root.Lstat(unit)
	if err != nil || !trustedDirectory(before) {
		return nil, errors.Join(errBinding, err)
	}
	directory, err = root.OpenFile(unit, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	opened, err := directory.Stat()
	var filesystem syscall.Statfs_t
	filesystemErr := syscall.Fstatfs(int(directory.Fd()), &filesystem)
	if err != nil || !trustedDirectory(opened) || !os.SameFile(before, opened) ||
		filesystemErr != nil || filesystem.Type != cgroup2Magic {
		return nil, errors.Join(errBinding, err, filesystemErr)
	}
	descriptor, err := syscall.Openat(int(directory.Fd()), "cgroup.events", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	events = os.NewFile(uintptr(descriptor), "installation cgroup.events")
	identity, err := events.Stat()
	if err != nil {
		return nil, err
	}
	observed := &pin{events: events, identity: identity}
	removed, _, err := observed.read()
	if err != nil || removed {
		return nil, errors.Join(errBinding, err)
	}
	return observed, nil
}

func (pin *pin) read() (removed, populated bool, returnedErr error) {
	if pin == nil || pin.events == nil || pin.identity == nil {
		return false, false, errInput
	}
	info, err := pin.events.Stat()
	if err != nil || !os.SameFile(pin.identity, info) || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false, false, errors.Join(errBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	var filesystem syscall.Statfs_t
	filesystemErr := syscall.Fstatfs(int(pin.events.Fd()), &filesystem)
	if !ok || native.Uid != 0 || native.Gid != 0 || filesystemErr != nil || filesystem.Type != cgroup2Magic {
		return false, false, errors.Join(errBinding, filesystemErr)
	}
	// ENODEV is completion evidence only on this verified original core file.
	// Directory nlink, path absence and arbitrary filesystem errors are not join.
	if _, err := pin.events.Seek(0, io.SeekStart); err != nil {
		if errors.Is(err, syscall.ENODEV) {
			return true, false, nil
		}
		return false, false, err
	}
	body, err := io.ReadAll(io.LimitReader(pin.events, 1025))
	if errors.Is(err, syscall.ENODEV) && len(body) == 0 {
		return true, false, nil
	}
	if err != nil {
		return false, false, err
	}
	populated, err = parseEvents(body)
	return false, populated, err
}

func (pin *pin) close() error {
	if pin == nil || pin.events == nil {
		return nil
	}
	err := pin.events.Close()
	pin.events = nil
	return err
}

// This bounded inventory owns no stop authority. Each original events object
// must be pinned, and actual invocation reobserved, before a live stop request.
func scopeNames(ctx context.Context) (names []string, returnedErr error) {
	if ctx == nil {
		return nil, errInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	const directory = "/sys/fs/cgroup/system.slice"
	if err := observeAncestors(directory); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err())
		if returnedErr != nil {
			names = nil
		}
	}()
	return readScopeNames(ctx, root)
}

// Even an empty inventory must originate on the actual selected kernel
// filesystem. An ordinary empty directory supplies no quiescence evidence.
func readScopeNames(ctx context.Context, root *os.Root) (names []string, returnedErr error) {
	if ctx == nil || root == nil {
		return nil, errInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err())
		if returnedErr != nil {
			names = nil
		}
	}()
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &filesystem); err != nil || filesystem.Type != cgroup2Magic {
		return nil, errors.Join(errBinding, err)
	}
	entries, readErr := file.Readdirnames(4097)
	if len(entries) > 4096 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		return nil, errors.Join(errBinding, readErr)
	}
	for _, name := range entries {
		if name != "ardents-endpoint.service" && !strings.HasPrefix(name, "ardents-text-reader@") && !strings.HasPrefix(name, "ardents-text-publisher@") {
			continue
		}
		if !scopeUnit(name) || len(names) >= 129 {
			return nil, errBinding
		}
		names = append(names, name)
	}
	return names, nil
}

func retainPins(ctx context.Context, pid, uid uint32, retainPartial bool) (pins []*pin, returnedErr error) {
	if ctx == nil || (pid == 0 && !retainPartial) || uid == 0 {
		return nil, errInput
	}
	defer func() {
		if returnedErr != nil && !retainPartial {
			for _, pin := range pins {
				returnedErr = errors.Join(returnedErr, pin.close())
			}
			pins = nil
		}
	}()
	endpoint, err := pinScope(ctx, "ardents-endpoint.service")
	if err != nil {
		return nil, err
	}
	pins = append(pins, endpoint)
	names, err := scopeNames(ctx)
	if err != nil {
		return pins, err
	}
	for _, name := range names {
		if name == "ardents-endpoint.service" {
			continue
		}
		if !workerMatches(name, pid, uid) {
			return pins, errBinding
		}
		pin, err := pinScope(ctx, name)
		if err != nil {
			return pins, err
		}
		pins = append(pins, pin)
	}
	return pins, ctx.Err()
}

// Absence of a pathname and a newly empty replacement scope cannot complete
// the original lifetime. Every retained original kernel object is observed.
func pinsJoined(pins []*pin) (bool, error) {
	if len(pins) == 0 {
		return false, errInput
	}
	joined := true
	var observationErr error
	for _, pin := range pins {
		removed, populated, err := pin.read()
		observationErr = errors.Join(observationErr, err)
		joined = joined && err == nil && (removed || !populated)
	}
	return joined, observationErr
}

// Lifetime retains original kernel events descriptors, not stop authority.
// After requesting stop, its caller must establish Joined before Close.
type Lifetime struct{ pins []*pin }

// Retain pins the closed Installation scope set for the supplied process and
// account. The caller separately proves actual process and manager invocation.
func Retain(ctx context.Context, pid, uid uint32) (*Lifetime, error) {
	pins, err := retainPins(ctx, pid, uid, false)
	if err != nil {
		return nil, err
	}
	return &Lifetime{pins: pins}, nil
}

// RetainStarted preserves an original Endpoint events descriptor even if a
// subsequent worker inventory or cancellation refuses. After a manager start
// attempt, failure cannot release that partial custody before physical join.
// A nonnil result with an error owns those descriptors and grants no admission.
// PID zero is failure-only pending-start observation: no worker scope can match
// it. The caller still proves its own attempted fixed-unit transition.
func RetainStarted(ctx context.Context, pid, uid uint32) (*Lifetime, error) {
	pins, err := retainPins(ctx, pid, uid, true)
	if len(pins) == 0 {
		return nil, err
	}
	return &Lifetime{pins: pins}, err
}

// Observe refuses a retired original descriptor without adopting another path.
func (lifetime *Lifetime) Observe() error {
	if lifetime == nil || len(lifetime.pins) == 0 {
		return errInput
	}
	for _, pin := range lifetime.pins {
		removed, _, err := pin.read()
		if err != nil || removed {
			return errors.Join(errBinding, err)
		}
	}
	return nil
}

// Joined requires every original kernel object to be empty or removed.
func (lifetime *Lifetime) Joined() (bool, error) {
	if lifetime == nil {
		return false, errInput
	}
	return pinsJoined(lifetime.pins)
}

// Close releases observations only; it neither stops processes nor joins them.
func (lifetime *Lifetime) Close() error {
	if lifetime == nil {
		return nil
	}
	var result error
	for _, pin := range lifetime.pins {
		result = errors.Join(result, pin.close())
	}
	lifetime.pins = nil
	return result
}

// ObserveEmpty checks actual kernel inventory even when there are no names.
// It grants no manager or invocation authority and starts or stops no process.
func ObserveEmpty(ctx context.Context) error {
	names, err := scopeNames(ctx)
	if err != nil {
		return err
	}
	for _, name := range names {
		pin, err := pinScope(ctx, name)
		if err != nil {
			return err
		}
		removed, populated, readErr := pin.read()
		closeErr := pin.close()
		if readErr != nil || closeErr != nil || (!removed && populated) {
			return errors.Join(errBinding, readErr, closeErr)
		}
	}
	return ctx.Err()
}

// The fixed kernel path requires direct root-owned, non-writable ancestors.
func trustedDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func observeAncestors(directory string) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return errBinding
	}
	for {
		info, err := os.Lstat(directory)
		if err != nil || !trustedDirectory(info) {
			return errors.Join(errBinding, err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return nil
		}
		directory = parent
	}
}
