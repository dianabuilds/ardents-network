package resource

import (
	"path"
	"strings"
)

const qualificationOwnerCgroupRoot = "/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice/"

func validOwnerWorkerCgroup(group string) bool {
	if path.Clean(group) != group || strings.ContainsAny(group, "\x00\r\n") {
		return false
	}
	if strings.HasPrefix(group, qualificationOwnerCgroupRoot) {
		return !strings.Contains(strings.TrimPrefix(group, qualificationOwnerCgroupRoot), "/")
	}
	return strings.HasPrefix(group, "/system.slice/") && !strings.Contains(strings.TrimPrefix(group, "/system.slice/"), "/")
}
