package process

import (
	"strings"
	"testing"
)

func TestInstallationProcessStartRetainsOriginalPIDAndClock(t *testing.T) {
	// comm may itself contain spaces and parentheses; starttime is field 22.
	stat := "123 (name ) with spaces) S " + strings.Repeat("0 ", 18) + "456 0\n"
	if started, err := startClock([]byte(stat), 123); err != nil || started != 456 {
		t.Fatalf("original start clock differs: %d %v", started, err)
	}
	for _, body := range []string{
		strings.Replace(stat, "123 (", "124 (", 1),
		strings.Replace(stat, ") S ", ") Z ", 1),
		strings.Replace(stat, "456", "0", 1),
		strings.Replace(stat, "456", "0456", 1),
		"123 (truncated) S 1\n",
	} {
		if _, err := startClock([]byte(body), 123); err == nil {
			t.Fatal("foreign/dead/incomplete process observation accepted")
		}
	}
}

func TestInstallationProcessCredentialsRefuseKernelPrivilegeMismatch(t *testing.T) {
	status := "Name:\tfixture\nUid:\t996 996 996 996\nGid:\t988 988 988 988\nGroups:\t988\nNoNewPrivs:\t1\nCapInh:\t0000000000000000\nCapPrm:\t0000000000000000\nCapEff:\t0000000000000000\nCapBnd:\t0000000000000000\nCapAmb:\t0000000000000000\n"
	if err := credentials([]byte(status), 996, 988); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		strings.Replace(status, "996 996 996 996", "996 0 996 996", 1),
		strings.Replace(status, "988 988 988 988", "988 988 0 988", 1),
		strings.Replace(status, "NoNewPrivs:\t1", "NoNewPrivs:\t0", 1),
		strings.Replace(status, "CapEff:\t0000000000000000", "CapEff:\t0000000000000001", 1),
		strings.Replace(status, "CapAmb:\t0000000000000000\n", "", 1),
		status + "Uid:\t996 996 996 996\n",
		strings.Replace(status, "Groups:\t988\n", "", 1),
		strings.Replace(status, "Groups:\t988", "Groups:\t988 0", 1),
		strings.Replace(status, "Groups:\t988", "Groups:\t0988", 1),
		status + "Groups:\t988\n",
	} {
		if err := credentials([]byte(body), 996, 988); err == nil {
			t.Fatal("changed or incomplete kernel credentials accepted")
		}
	}
	if err := credentials([]byte(strings.Replace(status, "Groups:\t988", "Groups:\t", 1)), 996, 988); err != nil {
		t.Fatal("empty supplementary group set refused", err)
	}
}

func TestInstallationProcessInvocationRequiresExactSingleEnvironmentValue(t *testing.T) {
	invocation := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	environment := "PATH=/usr/bin\x00INVOCATION_ID=0102030405060708090a0b0c0d0e0f10\x00"
	if !invocationMatches([]byte(environment), invocation) {
		t.Fatal("original invocation refused")
	}
	for _, body := range []string{
		"PATH=/usr/bin\x00",
		strings.TrimSuffix(environment, "\x00"),
		strings.Replace(environment, "010203", "090203", 1),
		environment + "INVOCATION_ID=0102030405060708090a0b0c0d0e0f10\x00",
	} {
		if invocationMatches([]byte(body), invocation) {
			t.Fatal("missing/torn/replacement/duplicate invocation accepted")
		}
	}
}
