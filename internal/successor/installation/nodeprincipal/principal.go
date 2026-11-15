package nodeprincipal

import (
	"errors"
	"strconv"
	"strings"
)

// ErrBinding reports a malformed or inconsistent fixed Node principal.
var ErrBinding = errors.New("invalid fixed Node principal")

// Delta validates the exact account-file change for the fixed Node.
func Delta(path string, before, after []byte, uid, gid uint32) error {
	if len(before) == 0 || before[len(before)-1] != '\n' || len(after) == 0 || after[len(after)-1] != '\n' {
		return ErrBinding
	}
	var own string
	var rest strings.Builder
	for _, line := range strings.SplitAfter(string(after), "\n") {
		if strings.HasPrefix(line, "ardents-node:") {
			if own != "" {
				return ErrBinding
			}
			own = strings.TrimSuffix(line, "\n")
		} else {
			rest.WriteString(line)
		}
	}
	if rest.String() != string(before) || own == "" {
		return ErrBinding
	}
	return ValidateRow(path, own, uid, gid)
}

// The initial creator checks its foreign-byte delta separately. Later native
// admission independently checks the same fixed principal's current rows; this
// pure condition supplies neither creation provenance nor account ownership.
// ValidateRow validates one fixed Node account row.
func ValidateRow(path, own string, uid, gid uint32) error {
	if !strings.HasPrefix(own, "ardents-node:") {
		return ErrBinding
	}
	u, g := strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(gid), 10)
	switch path {
	case "/etc/passwd":
		if own != "ardents-node:x:"+u+":"+g+"::/nonexistent:/usr/sbin/nologin" {
			return ErrBinding
		}
	case "/etc/group":
		if own != "ardents-node:x:"+g+":" {
			return ErrBinding
		}
	case "/etc/gshadow":
		if own != "ardents-node:!::" {
			return ErrBinding
		}
	case "/etc/shadow":
		f := strings.Split(own, ":")
		if len(f) != 9 || f[1] != "!" {
			return ErrBinding
		}
		if _, e := strconv.ParseUint(f[2], 10, 32); e != nil {
			return ErrBinding
		}
		for _, v := range f[3:] {
			if v != "" {
				return ErrBinding
			}
		}
	default:
		return ErrBinding
	}
	return nil
}

// CurrentRow finds the unique fixed Node row in a complete database.
func CurrentRow(raw []byte) (string, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || strings.IndexByte(string(raw), 0) >= 0 {
		return "", ErrBinding
	}
	var own string
	for _, line := range strings.Split(string(raw), "\n") {
		name, _, _ := strings.Cut(line, ":")
		if name != "ardents-node" {
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

// ValidatePublic checks public account databases and NSS configuration.
func ValidatePublic(passwd, group, nss []byte, uid, gid uint32) (map[string]string, error) {
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
		row, err := CurrentRow(database.raw)
		if err != nil || ValidateRow(database.path, row, uid, gid) != nil {
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
			if err != nil || parts[0] != "ardents-node" && number == uint64(uid) {
				return nil, ErrBinding
			}
			if database.path == "/etc/passwd" {
				number, err = strconv.ParseUint(parts[3], 10, 32)
				if err != nil || parts[0] != "ardents-node" && number == uint64(gid) {
					return nil, ErrBinding
				}
			} else {
				for _, member := range strings.Split(parts[3], ",") {
					if member == "ardents-node" {
						return nil, ErrBinding
					}
				}
			}
		}
	}
	if err := ValidateNSS(nss); err != nil {
		return nil, err
	}
	return rows, nil
}

// ValidateNSS checks the admitted local NSS modules.
func ValidateNSS(raw []byte) error {
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

// SelectID chooses the highest free fixed system identity from the observed
// passwd and group preimages. The caller retains and rechecks those originals;
// the selected number grants no account creation or installation authority.
func SelectID(passwd, group []byte) (uint32, error) {
	used := map[uint32]bool{}
	for _, database := range []struct {
		raw    []byte
		passwd bool
	}{{passwd, true}, {group, false}} {
		for _, line := range strings.Split(string(database.raw), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Split(line, ":")
			if len(fields) < 3 || fields[0] == "ardents-node" {
				return 0, ErrBinding
			}
			n, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				return 0, ErrBinding
			}
			used[uint32(n)] = true
			if database.passwd {
				if len(fields) != 7 {
					return 0, ErrBinding
				}
				primary, err := strconv.ParseUint(fields[3], 10, 32)
				if err != nil {
					return 0, ErrBinding
				}
				used[uint32(primary)] = true
			}
		}
	}
	for id := uint32(999); id >= 100; id-- {
		if !used[id] {
			return id, nil
		}
	}
	return 0, ErrBinding
}
