package resource

import "testing"

func TestOwnerWorkerCgroupAcceptsOnlyMaintainedRoots(t *testing.T) {
	for _, path := range []string{"/system.slice/ardents-text-reader@1.service", qualificationOwnerCgroupRoot + "ardents-stream-qualification-reader@1.service"} {
		if !validOwnerWorkerCgroup(path) {
			t.Fatalf("maintained worker cgroup refused: %q", path)
		}
	}
	for _, path := range []string{"/foreign/worker.service", "/system.slice/foreign.scope/ardents-text-reader@1.service", "/foreign" + qualificationOwnerCgroupRoot + "worker.service", qualificationOwnerCgroupRoot + "nested/worker.service", qualificationOwnerCgroupRoot + "../worker.service"} {
		if validOwnerWorkerCgroup(path) {
			t.Fatalf("foreign worker cgroup accepted: %q", path)
		}
	}
}
