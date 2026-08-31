//go:build !linux && endpoint_portable_qualification

package endpoint_test

import "testing"

func TestUbuntuPortableUserUnitQualificationRequiresLinux(t *testing.T) {
	t.Fatal("portable Endpoint qualification requires an Ubuntu Linux systemd --user host")
}
