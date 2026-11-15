package main

import (
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
	"testing"
	"time"
)

func TestIntroductionHostingArgumentsKeepFinitePurposeAndRejectDuplicates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	args := []string{"--binding", "/opt/introduction/binding.json", "--provider", "declared-provider", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "17", "--low-watermark-bytes", "1024"}
	binding, policy, err := nodeHostingArgumentsForIntroduction(args)
	if err != nil || binding != "/opt/introduction/binding.json" || policy.InitialUsedBytes != 17 {
		t.Fatal("fixed public declaration refused", err)
	}
	for _, extra := range [][]string{{"--quantity", "2"}, {"--binding", "/other/binding.json"}, {"--role", "descriptor"}, {"--interfaces", "lo"}, {"trailing"}} {
		if _, _, err := nodeHostingArgumentsForIntroduction(append(append([]string(nil), args...), extra...)); err == nil {
			t.Fatal("ambiguous or foreign operation accepted", extra)
		}
	}
	changed := append([]string(nil), args...)
	changed[1] = "relative/binding.json"
	if _, _, err := nodeHostingArgumentsForIntroduction(changed); err == nil {
		t.Fatal("relative original binding accepted")
	}
	changed = append([]string(nil), args...)
	changed[5] = now.Format("2006-01-02T15:04:05.000Z")
	if _, _, err := nodeHostingArgumentsForIntroduction(changed); err == nil {
		t.Fatal("noncanonical time accepted")
	}
}

func nodeHostingArgumentsForIntroduction(args []string) (string, hosting.Policy, error) {
	return nodeHostingArguments(args, "introduction", introduction.ErrInput)
}
