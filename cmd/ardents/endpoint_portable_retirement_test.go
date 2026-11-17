package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The per-user Portable Endpoint lifecycle refuses before input or root
// effects (ADR-0125, #617 S3.5). Each former route returns its typed
// retirement refusal naming the canonical ardents-next successor path; no
// enrollment, replacement, headless runtime, user unit or rollback effect
// remains reachable through this command.
func TestLegacyEndpointPortableLifecycleRetiredBeforeEffects(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "retained-state")
	original := []byte("retained application data and trust history\n")
	if err := os.WriteFile(record, original, 0600); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "bundle")
	pin := strings.Repeat("a", 64)
	for _, route := range []struct {
		operation string
		refusal   string
		arguments [][]string
	}{
		{"enroll", "endpoint enroll is retired", [][]string{
			{"endpoint", "enroll"}, {"endpoint", "enroll", bundle}, {"endpoint", "enroll", bundle, pin},
			{"endpoint", "enroll", bundle, pin, "extra"}}},
		{"enroll-installed", "endpoint enroll-installed is retired", [][]string{
			{"endpoint", "enroll-installed"}, {"endpoint", "enroll-installed", filepath.Join(root, "package-enrollment.json")}}},
		{"headless", "endpoint headless is retired", [][]string{
			{"endpoint", "headless"}, {"endpoint", "headless", filepath.Join(root, "headless-runtime.json")}}},
		{"user-unit", "endpoint user-unit is retired", [][]string{
			{"endpoint", "user-unit"}, {"endpoint", "user-unit", bundle}, {"endpoint", "user-unit", bundle, pin}}},
		{"installed-user-unit", "endpoint installed-user-unit is retired", [][]string{
			{"endpoint", "installed-user-unit"}, {"endpoint", "installed-user-unit", filepath.Join(root, "package-enrollment.json")}}},
		{"replace", "endpoint replace is retired", [][]string{
			{"endpoint", "replace"}, {"endpoint", "replace", bundle}}},
		{"rollback", "endpoint rollback is retired", [][]string{
			{"endpoint", "rollback"}, {"endpoint", "rollback", bundle}}},
		{"replacement-self-test", "endpoint replacement-self-test is retired", [][]string{
			{"endpoint", "replacement-self-test"}, {"endpoint", "replacement-self-test", filepath.Join(root, "replacement")}}},
		{"replacement-recovery", "endpoint replacement-recovery is retired", [][]string{
			{"endpoint", "replacement-recovery"}, {"endpoint", "replacement-recovery", "extra"}}},
	} {
		for _, args := range route.arguments {
			var out bytes.Buffer
			err := runEndpoint(context.Background(), args, &out)
			if err == nil || !strings.Contains(err.Error(), route.refusal) || out.Len() != 0 {
				t.Fatalf("retired %s reached effects: %v %s", route.operation, err, out.String())
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("retired command changed root: %v %v", entries, err)
	}
	after, err := os.ReadFile(record)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("retired command changed retained data: %v", err)
	}
}
