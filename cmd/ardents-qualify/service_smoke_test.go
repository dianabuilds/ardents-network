package main

import "testing"

func TestServiceSmokeRejectsUnexpectedPositionals(t *testing.T) {
	if _, err := evaluateServiceSmoke([]string{"unexpected"}); err == nil {
		t.Fatal("service-smoke accepted an unexpected positional argument")
	}
}
