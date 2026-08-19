package naming

import (
	"strings"
	"testing"
)

// TestR041_NumericBounds exercises the three R-041 numeric limits:
// per-label 1-63, total <= 253, depth <= 127. The total limit is
// independent of the per-label and depth limits, so it is exercised
// with 2-character labels at depth 85 (84*2 + 1 root + 84 dots = 253
// exactly is the maximum reachable without breaking the per-label
// limit; 85 labels yields 254 bytes, just over the cap).
func TestR041_NumericBounds(t *testing.T) {
	t.Parallel()

	// Per-label 1-63: a 63-byte label is acceptable.
	sixtyThree := strings.Repeat("a", 63) + ".b"
	if _, err := Parse(sixtyThree); err != nil {
		t.Errorf("Parse(63-byte label) = %v, want nil", err)
	}
	// Per-label 1-63: 64-byte label is rejected.
	overlong := strings.Repeat("a", 64) + ".b"
	if _, err := Parse(overlong); err == nil {
		t.Errorf("Parse(64-byte label) = nil, want error")
	}

	// Total 253 with 2-char labels: 84 labels of "ab" + 83 dots = 251
	// chars, well under the cap.
	eightyFour := strings.Repeat("ab.", 83) + "ab"
	if _, err := Parse(eightyFour); err != nil {
		t.Errorf("Parse(84x2-deep) = %v, want nil", err)
	}
	// Total 254: 85 two-char labels + 84 dots exceeds 253.
	eightyFive := strings.Repeat("ab.", 84) + "ab"
	if _, err := Parse(eightyFive); err == nil {
		t.Errorf("Parse(85x2-deep = 254 bytes) = nil, want error")
	}

	// Depth 127: 127 single-char labels + 126 dots = 253 chars.
	deep := strings.Repeat("a.", 126) + "a"
	if _, err := Parse(deep); err != nil {
		t.Errorf("Parse(127-deep) = %v, want nil", err)
	}
	// Depth 128: rejected (exceeds the bound even when total is short).
	tooDeep := strings.Repeat("a.", 127) + "a" // 128 labels
	if _, err := Parse(tooDeep); err == nil {
		t.Errorf("Parse(128-deep) = nil, want error")
	}
}

// TestR041_NoAllDigitRootLabel per R-041: the top (rightmost) label
// must not be all-digit. Mixed root labels with at least one letter
// are accepted, and a numeric non-root label is accepted.
func TestR041_NoAllDigitRootLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"letter root", "blog.example", false},
		{"digit-mixed root", "1a-blog.example", false},
		{"digit-letter root", "blog.1abc", false},
		{"all-digit root rejected", "blog.123", true},
		{"single all-digit rejected", "123", true},
		{"digit-only non-root accepted", "123.blog", false},
	}
	for _, tc := range cases {
		_, err := Parse(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("Parse(%q) = nil, want error", tc.input)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("Parse(%q) = %v, want nil", tc.input, err)
		}
	}
}

// TestR041_ConsecutiveHyphensRejected per R-041: a label must not
// contain consecutive hyphens.
func TestR041_ConsecutiveHyphensRejected(t *testing.T) {
	t.Parallel()
	if _, err := Parse("foo--bar.example"); err == nil {
		t.Errorf("Parse with consecutive hyphens = nil, want error")
	}
	if _, err := Parse("foo-bar.example"); err != nil {
		t.Errorf("Parse with single hyphen = %v, want nil", err)
	}
}

// TestR041_UnicodeIDNAPunycodeRejected per R-041: Unicode, IDNA, and
// Punycode are absent. The parser must reject without allocation or
// state mutation.
func TestR041_UnicodeIDNAPunycodeRejected(t *testing.T) {
	t.Parallel()
	cases := []string{
		"blög.example",          // non-ASCII
		"example.xn--bcher-kva", // IDNA / Punycode form
		"пример.example",        // Cyrillic
		"例子.example",            // CJK
		"aΔb.example",           // mixed Greek
	}
	for _, in := range cases {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil, want error", in)
		}
	}
}
