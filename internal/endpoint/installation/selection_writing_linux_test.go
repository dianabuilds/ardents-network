//go:build linux

package installation

import (
	"strings"
	"testing"
)

func TestStoppedUnitRefusesActiveOrForeignManagerProperties(t *testing.T) {
	name := "ardents-endpoint.service"
	body := "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/" + name + "\nDropInPaths=\nMainPID=0\n"
	if err := verifyStoppedUnit(body, name); err != nil {
		t.Fatal(err)
	}
	for label, changed := range map[string]string{
		"active":           strings.Replace(body, "ActiveState=inactive", "ActiveState=active", 1),
		"process":          strings.Replace(body, "MainPID=0", "MainPID=42", 1),
		"foreign fragment": strings.Replace(body, "/etc/systemd/system/", "/run/systemd/system/", 1),
		"drop-in":          strings.Replace(body, "DropInPaths=", "DropInPaths=/etc/systemd/system/override.conf", 1),
		"incomplete":       strings.Replace(body, "MainPID=0\n", "", 1),
		"duplicate":        body + "MainPID=0\n",
	} {
		t.Run(label, func(t *testing.T) {
			if err := verifyStoppedUnit(changed, name); err == nil {
				t.Fatal("unsafe manager observation accepted")
			}
		})
	}
}
