package resource

import (
	"strconv"
	"strings"
)

func socketDescriptorTarget(target string) bool {
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return false
	}
	inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
	_, err := strconv.ParseUint(inode, 10, 64)
	return inode != "" && err == nil
}
