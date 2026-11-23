package process

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"
)

func sourceRecoveryProcMount(body []byte) error {
	found := false
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[4] != "/proc" {
			continue
		}
		if found || len(fields) < 10 || fields[3] != "/" {
			return errBinding
		}
		found = true
		separator := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				separator = i
				break
			}
		}
		if separator < 0 || len(fields) != separator+4 || fields[separator+1] != "proc" {
			return errBinding
		}
		for _, options := range []string{fields[5], fields[separator+3]} {
			for option := range strings.SplitSeq(options, ",") {
				name, value, _ := strings.Cut(option, "=")
				if name == "subset" || name == "hidepid" && value != "0" && value != "off" {
					return errBinding
				}
			}
		}
	}
	if !found {
		return errBinding
	}
	return nil
}

// A real/effective/saved/filesystem UID match conservatively retains the
// principal. Missing, duplicated or malformed native fields are not absence.
func sourceRecoveryPrincipal(body []byte, uid uint32) (bool, error) {
	found, matched := false, false
	for line := range strings.SplitSeq(string(body), "\n") {
		name, value, _ := strings.Cut(line, ":")
		if name != "Uid" {
			continue
		}
		if found {
			return false, errBinding
		}
		found = true
		fields := strings.Fields(value)
		if len(fields) != 4 {
			return false, errBinding
		}
		for _, field := range fields {
			n, err := strconv.ParseUint(field, 10, 32)
			if err != nil || strconv.FormatUint(n, 10) != field {
				return false, errBinding
			}
			matched = matched || uint32(n) == uid
		}
	}
	if !found {
		return false, errBinding
	}
	return matched, nil
}

func startClock(body []byte, pid uint32) (uint64, error) {
	text := string(body)
	begin, end := strings.Index(text, " ("), strings.LastIndex(text, ") ")
	if begin < 1 || end <= begin || text[:begin] != strconv.FormatUint(uint64(pid), 10) {
		return 0, errBinding
	}
	fields := strings.Fields(text[end+2:])
	if len(fields) < 20 || !strings.Contains("RSDITt", fields[0]) || len(fields[0]) != 1 {
		return 0, errBinding
	}
	started, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || started == 0 || strconv.FormatUint(started, 10) != fields[19] {
		return 0, errBinding
	}
	return started, nil
}

func credentials(body []byte, uid, gid uint32) error {
	if uid == 0 || gid == 0 {
		return errBinding
	}
	wanted := map[string][]string{
		"Uid":        {strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(uid), 10)},
		"Gid":        {strconv.FormatUint(uint64(gid), 10), strconv.FormatUint(uint64(gid), 10), strconv.FormatUint(uint64(gid), 10), strconv.FormatUint(uint64(gid), 10)},
		"NoNewPrivs": {"1"}, "CapInh": {"0000000000000000"}, "CapPrm": {"0000000000000000"}, "CapEff": {"0000000000000000"}, "CapBnd": {"0000000000000000"}, "CapAmb": {"0000000000000000"},
	}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(string(body), "\n") {
		name, value, _ := strings.Cut(line, ":")
		if name == "Groups" {
			if seen[name] {
				return errBinding
			}
			for group := range strings.FieldsSeq(value) {
				if group != strconv.FormatUint(uint64(gid), 10) {
					return errBinding
				}
			}
			seen[name] = true
			continue
		}
		values, needed := wanted[name]
		if !needed {
			continue
		}
		if seen[name] || strings.Join(strings.Fields(value), " ") != strings.Join(values, " ") {
			return errBinding
		}
		seen[name] = true
	}
	if len(seen) != len(wanted)+1 {
		return errBinding
	}
	return nil
}

func invocationMatches(body []byte, invocation [16]byte) bool {
	if invocation == [16]byte{} || len(body) == 0 || body[len(body)-1] != 0 {
		return false
	}
	seen := false
	for item := range bytes.SplitSeq(body[:len(body)-1], []byte{0}) {
		if !bytes.HasPrefix(item, []byte("INVOCATION_ID=")) {
			continue
		}
		if seen || string(item[len("INVOCATION_ID="):]) != hex.EncodeToString(invocation[:]) {
			return false
		}
		seen = true
	}
	return seen
}
