package descriptor

import (
	"encoding/hex"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// A changed selection does not change the original native exit. The caller
// independently retains the complete retirement and exact selection lineage.
func verifyDescriptorReplacementStopped(unit, service systemd.Properties, b binding, old *startSnapshot, ret *retirementSnapshot) error {
	if old == nil || ret == nil || ret.sha256 == "" || !descriptorCleanExit(ret.result.execution(), old.archive) {
		return ErrBinding
	}
	if err := verifyDescriptorConfiguration(unit, service, b.Request, b.Generation); err != nil {
		return err
	}
	if verifyDescriptorStopped(unit, service, b.Request, b.Generation) == nil {
		return nil
	}
	if !systemd.Matches(unit, "ActiveState", "s", "inactive") || !systemd.Matches(unit, "SubState", "s", "dead") ||
		(!systemd.Matches(unit, "InvocationID", "ay", old.id) && !systemd.Matches(unit, "InvocationID", "ay", []any{})) ||
		!systemd.Matches(unit, "TriggeredBy", "as", []string{}) || !systemd.Matches(service, "MainPID", "u", uint32(0)) || !systemd.Matches(service, "ControlPID", "u", uint32(0)) || !systemd.Matches(service, "NRestarts", "u", uint32(0)) ||
		!systemd.Matches(service, "DynamicUser", "b", false) || !systemd.Matches(service, "StandardInput", "s", "null") ||
		(!systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-descriptor.service")) || old.archive.InvocationID != hex.EncodeToString(old.id[:]) {
		return ErrBinding
	}
	actual, err := descriptorExecutionStatus(service)
	if err != nil || actual != ret.result.execution() {
		return ErrBinding
	}
	return nil
}
