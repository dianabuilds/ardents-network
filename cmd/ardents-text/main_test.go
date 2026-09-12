package main

import "testing"

func TestCommandRejectsArbitraryEntrypointsAndArguments(t *testing.T) {
	for _, arguments := range [][]string{nil, {"shell"}, {"worker-reader", "destination"}, {"worker-publisher", "document-path"}} {
		if err := run(arguments); err == nil {
			t.Fatal("worker command accepted an unselected entrypoint or ambient input")
		}
	}
}
