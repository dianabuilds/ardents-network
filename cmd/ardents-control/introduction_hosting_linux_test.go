package main

import (
	"testing"
	"time"
)

func TestIntroductionHostingArgumentsKeepFinitePurposeAndRejectDuplicates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	args := []string{"--binding", "/opt/introduction/binding.json", "--provider", "declared-provider", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "17", "--low-watermark-bytes", "1024"}
	binding, policy, err := introductionHostingArguments(args)
	if err != nil || binding != "/opt/introduction/binding.json" || policy.InitialUsedBytes != 17 {
		t.Fatal("fixed public declaration refused", err)
	}
	for _, extra := range [][]string{{"--quantity", "2"}, {"--binding", "/other/binding.json"}, {"--role", "descriptor"}, {"--interfaces", "lo"}, {"trailing"}} {
		if _, _, err := introductionHostingArguments(append(append([]string(nil), args...), extra...)); err == nil {
			t.Fatal("ambiguous or foreign operation accepted", extra)
		}
	}
	changed := append([]string(nil), args...)
	changed[1] = "relative/binding.json"
	if _, _, err := introductionHostingArguments(changed); err == nil {
		t.Fatal("relative original binding accepted")
	}
	changed = append([]string(nil), args...)
	changed[5] = now.Format("2006-01-02T15:04:05.000Z")
	if _, _, err := introductionHostingArguments(changed); err == nil {
		t.Fatal("noncanonical time accepted")
	}
}
