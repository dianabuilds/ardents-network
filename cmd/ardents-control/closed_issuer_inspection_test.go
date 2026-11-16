package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestClosedIssuerInspectionRejectsUnverifiedInventoryWithoutOutput(t *testing.T) {
	now := time.Unix(2_000_401_600, 0).UTC().Truncate(time.Hour)
	network, node := [32]byte{1}, [32]byte{2}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	profile := signedPublicIssuerProfile(t, network, node, private, now)
	digest := sha256.Sum256(profile)
	type issuerExport struct {
		Schema  string `json:"schema"`
		Profile []byte `json:"profile"`
		Digest  string `json:"profile_sha256"`
	}
	valid := issuerExport{"ardents-closed-issuer-profile-v1", profile, hex.EncodeToString(digest[:])}
	encode := func(value issuerExport) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	validRaw := encode(valid)
	badSignature := valid
	badSignature.Profile = append([]byte(nil), profile...)
	badSignature.Profile[len(badSignature.Profile)-1] ^= 1
	changedDigest := sha256.Sum256(badSignature.Profile)
	badSignature.Digest = hex.EncodeToString(changedDigest[:])
	badDigest := valid
	badDigest.Digest = hex.EncodeToString(make([]byte, 32))
	badSchema := valid
	badSchema.Schema = "ardents-transit-issuer-profile-v1"
	badCohort := bytes.Clone(profile)
	badCohort[98] = 2
	copy(badCohort[len(badCohort)-ed25519.SignatureSize:], ed25519.Sign(private, append([]byte("ardents-closed-issuer-keys-v1\x00"), badCohort[:len(badCohort)-ed25519.SignatureSize]...)))
	for _, test := range []struct {
		name        string
		raw         []byte
		flag, value string
	}{
		{name: "wrong-signature-with-correct-digest", raw: encode(badSignature)},
		{name: "wrong-digest", raw: encode(badDigest)},
		{name: "wrong-schema", raw: encode(badSchema)},
		{name: "trailing-json", raw: append(append([]byte(nil), validRaw...), []byte("{}")...)},
		{name: "unknown-field", raw: append([]byte("{\"extra\":true,"), validRaw[1:]...)},
		{name: "oversized", raw: bytes.Repeat([]byte(" "), maximumClosedProfilePlanBytes+1)},
		{name: "wrong-network", raw: validRaw, flag: "--network", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "wrong-node", raw: validRaw, flag: "--node", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "wrong-key", raw: validRaw, flag: "--node-key", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "missing-key", raw: validRaw, flag: "--node-key", value: ""},
		{name: "raw-wrong-signature", raw: badSignature.Profile},
		{name: "raw-truncated", raw: profile[:len(profile)-1]},
		{name: "raw-trailing", raw: append(bytes.Clone(profile), 0)},
		{name: "raw-signed-invalid-cohort", raw: badCohort},
		{name: "raw-wrong-network", raw: profile, flag: "--network", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "raw-wrong-node", raw: profile, flag: "--node", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "raw-wrong-key", raw: profile, flag: "--node-key", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "public.json")
			if err := os.WriteFile(path, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			arguments := []string{"inspect-closed-issuer-profile", "--profile", path, "--network", hex.EncodeToString(network[:]),
				"--node", hex.EncodeToString(node[:]), "--node-key", hex.EncodeToString(private.Public().(ed25519.PublicKey))}
			for index := 1; index < len(arguments); index += 2 {
				if arguments[index] == test.flag {
					arguments[index+1] = test.value
				}
			}
			var output bytes.Buffer
			if err := run(arguments, &output); err == nil || output.Len() != 0 {
				t.Fatalf("unverified inventory produced output: %q / %v", output.String(), err)
			}
			retained, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(retained, test.raw) {
				t.Fatalf("inspection changed public input: %v", err)
			}
		})
	}
}

func TestPublicIssuerInspectionAcceptsNewRawProfileWithoutPrivateRoots(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "ardents-control")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(t.Context(), "go", "build", "-buildvcs=false", "-o", binary, "./cmd/ardents-control")
	build.Dir = repository
	build.Env = append(os.Environ(), "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real public inspection consumer: %v\n%s", err, output)
	}
	network, node := [32]byte{1}, [32]byte{2}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	start := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := signedPublicIssuerProfile(t, network, node, private, start)
	digest := sha256.Sum256(profile)
	export, err := json.Marshal(map[string]any{"schema": "ardents-closed-issuer-profile-v1", "profile": profile, "profile_sha256": hex.EncodeToString(digest[:])})
	if err != nil {
		t.Fatal(err)
	}
	var expected []byte
	for _, input := range [][]byte{export, profile} {
		root := t.TempDir()
		path := filepath.Join(root, "public-profile")
		if err := os.WriteFile(path, input, 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"inspect-closed-issuer-profile", "--profile", path, "--network", hex.EncodeToString(network[:]), "--node", hex.EncodeToString(node[:]), "--node-key", hex.EncodeToString(private.Public().(ed25519.PublicKey))}
		output, err := exec.CommandContext(t.Context(), binary, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("public-only offline inspection refused: %v\n%s", err, output)
		}
		if expected == nil {
			expected = bytes.Clone(output)
		}
		if !bytes.Equal(expected, output) {
			t.Fatal("raw profile and public export reports differ")
		}
		var report struct {
			Schema, ProfileSHA256, NotBefore, NotAfter string
			TokenKeys                                  []closedProfilePlanKey
		}
		if json.Unmarshal(output, &report) != nil || report.Schema != "ardents-closed-issuer-inspection-v1" || report.ProfileSHA256 != hex.EncodeToString(digest[:]) || report.NotBefore != start.Format(time.RFC3339) || report.NotAfter != start.Add(time.Hour).Format(time.RFC3339) || len(report.TokenKeys) != 3 {
			t.Fatalf("lost signed public inventory: %s", output)
		}
		for i, key := range report.TokenKeys {
			spki := profile[90+i*357+11 : 90+(i+1)*357]
			if key.Class != byte(i+1) || key.WindowStart != start.Format(time.RFC3339) || key.SPKI != base64.RawStdEncoding.EncodeToString(spki) {
				t.Fatal("reported cohort or exact SPKI changed", i)
			}
		}
		after, err := os.ReadFile(path)
		entries, listErr := os.ReadDir(root)
		if err != nil || listErr != nil || !bytes.Equal(input, after) || len(entries) != 1 {
			t.Fatal("inspection rewrote public input or created private state", err, listErr)
		}
	}
}
