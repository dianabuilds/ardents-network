package duty

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRootEntryClaimAndCandidateClauses guards the consolidated missing-marker
// clauses (audit NW-4): a claimed root may hold only the lock entry, while a
// candidate root must be empty. Both share the marker, entry-bound and allowlist
// refusals; the parameterization must not change either boundary.
func TestRootEntryClaimAndCandidateClauses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		entries     []string
		marker      bool
		create      bool
		claimOK     bool
		candidateOK bool
	}{
		{"vacant-create", nil, false, true, false, true},
		{"lock-create", []string{rootLockName}, false, true, true, false},
		{"vacant-no-create", nil, false, false, false, false},
		{"lock-no-create", []string{rootLockName}, false, false, false, false},
		{"initialized-current", []string{rootMarkerName, rootLockName, "current"}, true, false, true, true},
		{"initialized-staging", []string{rootMarkerName, rootLockName, ".stage-1"}, true, false, true, true},
		{"two-entries-create", []string{rootLockName, "current"}, false, true, false, false},
		{"unknown-entry", []string{rootMarkerName, "intruder"}, true, false, false, false},
		{"directory-entry", []string{rootMarkerName, "state-1"}, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.marker {
				if err := os.WriteFile(filepath.Join(root, rootMarkerName), []byte(rootMarker), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tc.entries {
				if name == rootMarkerName {
					continue // written with valid content above when tc.marker is set
				}
				path := filepath.Join(root, name)
				if name == "state-1" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			claimErr := verifyRootClaim(root, tc.create)
			candidateErr := verifyRootCandidate(root, tc.create)
			if (claimErr == nil) != tc.claimOK {
				t.Fatalf("claim ok=%v, want %v (err=%v)", claimErr == nil, tc.claimOK, claimErr)
			}
			if (candidateErr == nil) != tc.candidateOK {
				t.Fatalf("candidate ok=%v, want %v (err=%v)", candidateErr == nil, tc.candidateOK, candidateErr)
			}
		})
	}
}
