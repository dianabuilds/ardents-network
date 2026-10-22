package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Host databases remain host-owned. Each observation independently reads the
// current fixed principal; unrelated completed host updates are not frozen to
// the initial helper's postimages. Root additionally checks private lock rows
// and actual NSS results before its effect. Runtime verifies its own kernel
// credentials separately and spawns no NSS helper in the Source cgroup.
func observeSourcePrincipal(ctx context.Context, uid, gid uint32) (err error) {
	if ctx == nil || uid < 100 || uid > 999 || gid != uid {
		return ErrInput
	}
	files := map[string]*sourceFile{}
	defer func() {
		for _, file := range files {
			err = errors.Join(err, file.file.Close())
			clear(file.raw)
		}
		err = errors.Join(err, ctx.Err())
	}()
	paths := []string{"/etc/passwd", "/etc/group", "/etc/nsswitch.conf"}
	if os.Geteuid() == 0 {
		paths = append(paths, "/etc/shadow", "/etc/gshadow", "/usr/bin/getent")
	}
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return err
		}
		file, e := readSourceAccountFile(path)
		if e != nil {
			return e
		}
		files[path] = file
	}
	rows, err := validateSourcePublicPrincipal(files["/etc/passwd"].raw, files["/etc/group"].raw, files["/etc/nsswitch.conf"].raw, uid, gid)
	if err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		for _, path := range []string{"/etc/shadow", "/etc/gshadow"} {
			row, e := sourceCurrentPrincipalRow(files[path].raw)
			if e != nil || validateSourcePrincipalRow(path, row, uid, gid) != nil {
				return errors.Join(ErrBinding, e)
			}
		}
		for _, query := range []struct{ database, value string }{
			{"passwd", "ardents-source"}, {"passwd", strconv.FormatUint(uint64(uid), 10)},
			{"group", "ardents-source"}, {"group", strconv.FormatUint(uint64(gid), 10)},
		} {
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = checkSourceAccountFile(files["/usr/bin/getent"]); err != nil {
				return err
			}
			bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
			command := exec.CommandContext(bounded, "/usr/bin/getent", query.database, query.value)
			command.Env = []string{"LANG=C", "PATH=/usr/bin:/bin"}
			var output sourceAccountDiagnostic
			command.Stdout = &output
			runtime.LockOSThread()
			// The finite helper is PID1 of its own namespace. Kernel teardown
			// joins descendants before reap, including after caller death/kill.
			command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Cloneflags: syscall.CLONE_NEWPID}
			runErr := command.Run()
			runtime.UnlockOSThread()
			contextErr := bounded.Err()
			cancel()
			if runErr != nil || contextErr != nil || output.body.String() != rows[query.database]+"\n" {
				return errors.Join(ErrBinding, runErr, contextErr)
			}
		}
	}
	for _, file := range files {
		if err = checkSourceAccountFile(file); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func sourceCurrentPrincipalRow(raw []byte) (string, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || strings.IndexByte(string(raw), 0) >= 0 {
		return "", ErrBinding
	}
	var own string
	for _, line := range strings.Split(string(raw), "\n") {
		name, _, _ := strings.Cut(line, ":")
		if name != "ardents-source" {
			continue
		}
		if own != "" {
			return "", ErrBinding
		}
		own = line
	}
	if own == "" {
		return "", ErrBinding
	}
	return own, nil
}

func validateSourcePublicPrincipal(passwd, group, nss []byte, uid, gid uint32) (map[string]string, error) {
	if uid < 100 || uid > 999 || gid != uid {
		return nil, ErrBinding
	}
	rows := map[string]string{}
	for _, database := range []struct {
		path string
		raw  []byte
	}{
		{"/etc/passwd", passwd}, {"/etc/group", group},
	} {
		row, err := sourceCurrentPrincipalRow(database.raw)
		if err != nil || validateSourcePrincipalRow(database.path, row, uid, gid) != nil {
			return nil, errors.Join(ErrBinding, err)
		}
		rows[strings.TrimPrefix(database.path, "/etc/")] = row
		for _, line := range strings.Split(string(database.raw), "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Split(line, ":")
			count := 4
			if database.path == "/etc/passwd" {
				count = 7
			}
			if len(parts) != count {
				return nil, ErrBinding
			}
			number, err := strconv.ParseUint(parts[2], 10, 32)
			if err != nil || parts[0] != "ardents-source" && number == uint64(uid) {
				return nil, ErrBinding
			}
			if database.path == "/etc/passwd" {
				number, err = strconv.ParseUint(parts[3], 10, 32)
				if err != nil || parts[0] != "ardents-source" && number == uint64(gid) {
					return nil, ErrBinding
				}
			} else {
				for _, member := range strings.Split(parts[3], ",") {
					if member == "ardents-source" {
						return nil, ErrBinding
					}
				}
			}
		}
	}
	if err := validateSourceNSS(nss); err != nil {
		return nil, err
	}
	return rows, nil
}

func validateSourceNSS(raw []byte) error {
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line, _, _ = strings.Cut(line, "#")
		name, value, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if name != "passwd" && name != "group" {
			continue
		}
		if !ok || seen[name] {
			return ErrBinding
		}
		seen[name] = true
		modules := strings.Fields(value)
		if len(modules) == 0 {
			return ErrBinding
		}
		for _, module := range modules {
			if module != "files" && module != "systemd" {
				return ErrBinding
			}
		}
	}
	if len(seen) != 2 {
		return ErrBinding
	}
	return nil
}
