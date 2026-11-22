package main

import "testing"

func TestPublicOptionsAllowOnlyCommandRepeats(t *testing.T) {
	epoch := []string{"domain", "record-plan", "record"}
	profile := []string{"duty"}
	for _, test := range []struct {
		name       string
		args       []string
		repeatable []string
		want       bool
	}{
		{"unique and terminator", []string{"--network-id=one", "-output", "/out", "--"}, nil, true},
		{"mixed duplicate", []string{"--network-id", "one", "-network-id=one"}, nil, false},
		{"epoch inputs", []string{"--domain=a", "-domain", "b", "--record-plan", "/a", "--record-plan=/b", "--record=/a", "-record", "/b"}, epoch, true},
		{"epoch seed", []string{"--assignment-seed", "one", "--assignment-seed=two"}, epoch, false},
		{"epoch duty", []string{"--duty=one", "--duty=two"}, epoch, false},
		{"profile duties", []string{"--duty", "one", "-duty=two"}, profile, true},
		{"profile domain", []string{"--domain=a", "--domain=b"}, profile, false},
		{"bare dash values", []string{"--bundle", "-", "--token-inventory", "-"}, profile, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := uniquePublicOptions(test.args, test.repeatable...); got != test.want {
				t.Fatalf("option uniqueness = %v, want %v", got, test.want)
			}
		})
	}
}
