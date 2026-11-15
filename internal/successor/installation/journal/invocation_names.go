package journal

import (
	"fmt"
	"strconv"
	"strings"
)

// InvocationPrefix checks the exact contiguous names in a retained invocation
// namespace. It observes no filesystem, journal, lease or runtime authority;
// each Installation owner supplies its own fixed purpose and bound and remains
// responsible for independently opening and verifying every original record.
func InvocationPrefix(names []string, purpose string, limit uint32) (uint32, error) {
	if (purpose != "join" && purpose != "descriptor" && purpose != "forwarding" && purpose != "introduction") || limit == 0 || len(names) == 0 || uint64(len(names)) > uint64(limit) {
		return 0, ErrBinding
	}
	seen := make(map[uint32]bool, len(names))
	var latest uint32
	for _, name := range names {
		var number uint32
		if name != purpose+"-startup" {
			prefix := purpose + "-invocation-"
			n, err := strconv.ParseUint(strings.TrimPrefix(name, prefix), 10, 32)
			if err != nil || n == 0 || n >= uint64(limit) || name != fmt.Sprintf("%s%08d", prefix, n) {
				return 0, ErrBinding
			}
			number = uint32(n)
		}
		if seen[number] {
			return 0, ErrBinding
		}
		seen[number] = true
		latest = max(latest, number)
	}
	for n := uint32(0); n <= latest; n++ {
		if !seen[n] {
			return 0, ErrBinding
		}
	}
	return latest, nil
}
