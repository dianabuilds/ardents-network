//go:build !linux && h41aqualification

package endpoint_test

import "testing"

func TestUbuntuPortableUserUnitQualificationRequiresLinux(t *testing.T) {
	t.Fatal("H4-1A qualification requires an Ubuntu Linux systemd --user host")
}
