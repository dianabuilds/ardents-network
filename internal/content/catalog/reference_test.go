package catalog_test

import (
	"encoding/json"
	"strings"
	"testing"

	"ardents/internal/content/catalog"
)

const helloReference = "bafkreibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq"

func TestContentReferenceParsesCanonicalCIDAndIsComparable(t *testing.T) {
	reference, err := catalog.ParseContentReference(helloReference)
	if err != nil {
		t.Fatalf("ParseContentReference() error = %v", err)
	}
	if got := reference.String(); got != helloReference {
		t.Fatalf("String() = %q, want %q", got, helloReference)
	}
	if !reference.Equal(reference) {
		t.Fatal("Equal() = false for the same reference")
	}

	index := map[catalog.ContentReference]string{reference: "hello"}
	if got := index[reference]; got != "hello" {
		t.Fatalf("comparable map lookup = %q, want hello", got)
	}
}

func TestContentReferenceRejectsUnsupportedAndNonCanonicalCIDs(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"malformed":        "not-a-cid",
		"whitespace":       " " + helloReference,
		"CIDv0":            "QmYwAPJzv5CZsnAzt8auVZRnGiRA9i7VtkNQkKni1fT4Qg",
		"unknown version":  "bajkreibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq",
		"unknown codec":    "bafybeibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq",
		"unknown hash":     "bafkrgqe3ohjcjplc6n4f3fwunlj6upltggn7xqujbsvnvyw764srszz4u4rshq6ztos4chl4plgg4ffyyxnayrtdi5oc4xb2332g645433aeg",
		"zero digest":      "bafkreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"uppercase base32": strings.ToUpper(helloReference),
		"base36":           "k2cwue9rqdypmt3thjky14z1tk9fi9f0o5w7b3ofitdewlcf87lismqs",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if got, err := catalog.ParseContentReference(value); err == nil || got.String() != "" {
				t.Fatalf("ParseContentReference(%q) = (%q, %v), want invalid zero value", value, got.String(), err)
			}
		})
	}
}

func TestContentReferenceTextAndJSONCodecsAreStrictAndAtomic(t *testing.T) {
	reference, err := catalog.ParseContentReference(helloReference)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	text, err := reference.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if got := string(text); got != helloReference {
		t.Fatalf("MarshalText() = %q, want %q", got, helloReference)
	}
	var fromText catalog.ContentReference
	if err := fromText.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if !fromText.Equal(reference) {
		t.Fatal("text round trip changed reference")
	}

	encoded, err := json.Marshal(reference)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	if got, want := string(encoded), `"`+helloReference+`"`; got != want {
		t.Fatalf("MarshalJSON() = %s, want %s", got, want)
	}
	var fromJSON catalog.ContentReference
	if err := json.Unmarshal(encoded, &fromJSON); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if !fromJSON.Equal(reference) {
		t.Fatal("JSON round trip changed reference")
	}

	for name, raw := range map[string][]byte{
		"empty text":        nil,
		"malformed text":    []byte("not-a-cid"),
		"noncanonical text": []byte(strings.ToUpper(helloReference)),
	} {
		t.Run(name, func(t *testing.T) {
			target := reference
			if err := target.UnmarshalText(raw); err == nil {
				t.Fatal("UnmarshalText() error = nil, want rejection")
			}
			if !target.Equal(reference) {
				t.Fatal("failed text decode mutated receiver")
			}
		})
	}
	for name, raw := range map[string]string{
		"null":         `null`,
		"number":       `1`,
		"object":       `{}`,
		"malformed":    `"not-a-cid"`,
		"noncanonical": `"` + strings.ToUpper(helloReference) + `"`,
	} {
		t.Run("JSON "+name, func(t *testing.T) {
			target := reference
			if err := json.Unmarshal([]byte(raw), &target); err == nil {
				t.Fatal("UnmarshalJSON() error = nil, want rejection")
			}
			if !target.Equal(reference) {
				t.Fatal("failed JSON decode mutated receiver")
			}
		})
	}
}

func TestZeroContentReferenceIsInvalid(t *testing.T) {
	var zero catalog.ContentReference
	if zero.String() != "" {
		t.Fatalf("zero String() = %q, want empty", zero.String())
	}
	if _, err := zero.MarshalText(); err == nil {
		t.Fatal("zero MarshalText() error = nil, want rejection")
	}
	if _, err := json.Marshal(zero); err == nil {
		t.Fatal("zero JSON marshal error = nil, want rejection")
	}
}
