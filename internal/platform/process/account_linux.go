package process

import (
	"os"
	"strconv"
	"strings"
)

// DedicatedAccountID returns the common UID and GID of the named fixed service
// account from retained local passwd and group records. Both records must
// bind the same unprivileged ID and the required local account shape.
func DedicatedAccountID(passwd, group []byte, name string) (uint32, bool) {
	id := ""
	var uid uint32
	records := strings.Split(string(passwd), "\n")
	for _, line := range records {
		fields := strings.Split(line, ":")
		if len(fields) == 7 && fields[0] == name {
			if id != "" || fields[1] != "x" || fields[2] != fields[3] || fields[5] != "/nonexistent" || fields[6] != "/usr/sbin/nologin" {
				return 0, false
			}
			n, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil || n < 100 || n > 999 || strconv.FormatUint(n, 10) != fields[2] {
				return 0, false
			}
			id, uid = fields[2], uint32(n)
		}
	}
	if id == "" {
		return 0, false
	}
	for _, line := range records {
		fields := strings.Split(line, ":")
		if len(fields) == 7 && fields[0] != name && (fields[2] == id || fields[3] == id) {
			return 0, false
		}
	}
	found := false
	for _, line := range strings.Split(string(group), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			continue
		}
		if fields[0] == name {
			if found || fields[1] != "x" || fields[2] != id || fields[3] != "" {
				return 0, false
			}
			found = true
		} else if fields[2] == id {
			return 0, false
		}
		for _, member := range strings.Split(fields[3], ",") {
			if member == name {
				return 0, false
			}
		}
	}
	if !found {
		return 0, false
	}
	return uid, true
}

// RequireFixedServicePrincipal checks the invoking process's real and
// effective user and group IDs and supplementary groups.
func RequireFixedServicePrincipal() error {
	uid := os.Geteuid()
	if uid < 100 || uid > 999 || os.Getuid() != uid || os.Getgid() != uid || os.Getegid() != uid {
		return errInput
	}
	groups, err := os.Getgroups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group != uid {
			return errInput
		}
	}
	return nil
}
