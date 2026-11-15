package process

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func startClock(body []byte, pid uint32) (uint64, error) {
	clock, err := processresource.ParseStartClock(body, pid)
	if err != nil {
		return 0, errBinding
	}
	return clock, nil
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
	for _, line := range strings.Split(string(body), "\n") {
		name, value, _ := strings.Cut(line, ":")
		if name == "Groups" {
			if seen[name] {
				return errBinding
			}
			for _, group := range strings.Fields(value) {
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
	for _, item := range bytes.Split(body[:len(body)-1], []byte{0}) {
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
