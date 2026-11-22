package main

import "testing"

func TestCommandRejectsArbitraryEntrypointsAndArguments(t *testing.T) {
	for _, args := range [][]string{
		nil, {"read"}, {"link"}, {"publish"}, {"worker-reader", "extra"}, {"worker-publisher", "extra"},
	} {
		if err := run(args); err == nil || err.Error() != "document worker entrypoint is invalid" {
			t.Errorf("run(%q) = %v", args, err)
		}
	}
}
