package endpoint

import (
	"os"
	"strconv"
	"testing"
)

// LatencyDriverCounts selects the bounded short-run counts and declared seed.
func LatencyDriverCounts(t testing.TB) (cold, warm int, seed string) {
	t.Helper()
	seed = os.Getenv("ARDENTS_LATENCY_SEED")
	if seed == "" {
		seed = "seed-1"
	}
	cold = readLatencyCount(t, "ARDENTS_LATENCY_COLD")
	warm = readLatencyCount(t, "ARDENTS_LATENCY_WARM")
	return cold, warm, seed
}

func readLatencyCount(t testing.TB, name string) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return 3
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 || count > 1000 {
		t.Fatalf("%s must be an integer from 1 to 1000, got %q", name, value)
	}
	return count
}
