package main

import (
	"errors"
	"strings"
	"testing"
)

func TestNodePlanFlagsRejectAmbiguousOptions(t *testing.T) {
	invalid := errors.New("invalid")
	pin := strings.Repeat("1", 64)
	valid := []string{"--request", "/request", "--output", "/output", "--network-id", pin, "--node-id", pin, "--profile-authority", pin, "--authority", pin, "--threshold", "1", "--work-tx", "1", "--work-rx", "2", "--termination-tx", "3", "--termination-rx", "4", "--retirement-wait-ms", "100"}
	run := func(args []string) error {
		var threshold int
		var workTx, workRx, terminationTx, terminationRx uint64
		_, err := nodePlanFlags(args, "join", invalid, &threshold, &workTx, &workRx, &terminationTx, &terminationRx)
		return err
	}
	if err := run(valid); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{append(append([]string{}, valid...), "--output", "/other"), append(append([]string{}, valid...), "extra"), append([]string{}, valid[:len(valid)-2]...), append(append([]string{}, valid...), "--authority", "short")} {
		if err := run(args); !errors.Is(err, invalid) {
			t.Fatalf("ambiguous arguments accepted: %v", err)
		}
	}
}

func TestNodePlanKeysRequireCanonicalHex(t *testing.T) {
	invalid := errors.New("invalid")
	key := strings.Repeat("ab", 32)
	network, node, profile, pins, err := nodePlanKeys(key, key, key, []string{key}, invalid)
	if err != nil || network[0] != 0xab || node != network || profile != network || len(pins) != 1 || pins[0] != network {
		t.Fatalf("canonical inputs: %v", err)
	}
	for _, bad := range []string{strings.ToUpper(key), key[:62], strings.Repeat("xz", 32)} {
		if _, _, _, _, err := nodePlanKeys(key, key, key, []string{bad}, invalid); !errors.Is(err, invalid) {
			t.Fatalf("invalid pin admitted: %v", err)
		}
	}
}

func TestNodeOptionsAllowOnlyDeclaredRepeatedFlag(t *testing.T) {
	if !uniqueNodeOptions([]string{"--duty", "first", "--duty=second", "--output", "/tmp/result"}, "duty") {
		t.Fatal("declared repeated duty refused")
	}
	if uniqueNodeOptions([]string{"--duty", "first", "--duty=second", "--output", "/tmp/result", "-output=/tmp/other"}, "duty") {
		t.Fatal("duplicate output accepted under another spelling")
	}
	if uniqueNodeOptions([]string{"--duty", "first", "--duty=second"}, "") {
		t.Fatal("undeclared repeated duty accepted")
	}
}
