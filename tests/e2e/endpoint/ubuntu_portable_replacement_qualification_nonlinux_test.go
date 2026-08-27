//go:build !linux && h41bqualification

package endpoint_test

import "testing"

func TestUbuntuPortableReplacementQualificationRequiresLinux(t *testing.T) {
	t.Fatal("H4-1B qualification requires an Ubuntu Linux systemd --user host")
}
