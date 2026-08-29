package resource

import "testing"

func TestSocketDescriptorTargetAcceptsOnlyKernelSocketLinks(t *testing.T) {
	for _, test := range []struct {
		target string
		want   bool
	}{
		{target: "socket:[41731]", want: true},
		{target: "socket:[]", want: false},
		{target: "socket:[17]suffix", want: false},
		{target: "anon_inode:[eventpoll]", want: false},
		{target: "/var/lib/private/ardents-contributor/config/current/node.json", want: false},
	} {
		if got := socketDescriptorTarget(test.target); got != test.want {
			t.Fatalf("socketDescriptorTarget(%q) = %t, want %t", test.target, got, test.want)
		}
	}
}
