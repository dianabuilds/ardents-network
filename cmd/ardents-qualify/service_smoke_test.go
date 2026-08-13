package main

import "testing"

func TestServiceSmokeRejectsUnexpectedPositionals(t *testing.T) {
	if _, err := evaluate([]string{"service-smoke", "unexpected"}); err == nil {
		t.Fatal("service-smoke accepted an unexpected positional argument")
	}
}
