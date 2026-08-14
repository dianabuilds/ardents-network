package recoverysmoke

import (
	"strings"
	"testing"
)

func TestProcessIdentitySurvivesAStoppedContainer(t *testing.T) {
	container := strings.Repeat("a", 64)
	identity, err := parseProcessIdentity(container, []byte(container+" 2026-08-14T08:04:05.123456789Z\n"))
	if err != nil || identity != container+"@2026-08-14T08:04:05.123456789Z" {
		t.Fatalf("identity=%q err=%v", identity, err)
	}
	for name, raw := range map[string]string{
		"wrong container": strings.Repeat("b", 64) + " 2026-08-14T08:04:05Z",
		"zero start":      container + " 0001-01-01T00:00:00Z",
		"malformed":       container + " not-a-time",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseProcessIdentity(container, []byte(raw)); err == nil {
				t.Fatal("invalid process identity passed")
			}
		})
	}
}
